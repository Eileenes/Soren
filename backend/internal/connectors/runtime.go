package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/llm"
	"gorm.io/gorm"
)

var ErrApproval = errors.New("等待操作确认")
var ErrDenied = errors.New("用户拒绝操作，任务已停止")
var ErrUncertain = errors.New("外部操作结果待核实，任务已停止；不会自动重试")

type Service struct {
	PolicyLock   sync.RWMutex
	Policy       func(bot, run, action string) (bool, string, error)
	DB           *gorm.DB
	Vault        Vault
	HTTP         *http.Client
	MCPTransport http.RoundTripper // Injectable only for in-process connector tests.
	// Downloads from public signed URLs never carry connector credentials.
	DownloadHTTP         *http.Client
	DataDir              string
	Authorize            func(string) error
	GoogleAPI, NotionAPI string
	// APIBase overrides the default OpenAPI prefix per Baizhi connector kind (tests).
	APIBase map[string]string
	mu      sync.Mutex
	pending map[string]*oauthAttempt
}

func New(db *gorm.DB, v Vault, dir string) (*Service, error) {
	if e := migrate(db); e != nil {
		return nil, e
	}
	noRedirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Service{
		DB: db, Vault: v, DataDir: dir,
		HTTP:         &http.Client{Timeout: 45 * time.Second, CheckRedirect: noRedirect},
		DownloadHTTP: &http.Client{Transport: publicTransport(), Timeout: 2 * time.Minute, CheckRedirect: noRedirect},
		GoogleAPI:    "https://gmail.googleapis.com/gmail/v1", NotionAPI: "https://api.notion.com/v1",
		pending: map[string]*oauthAttempt{},
	}, nil
}

type binding struct {
	Connection Connection
	Action     string
	Tool       llm.Tool
	Approval   bool
}

func tool(name, desc string, properties map[string]any, required ...string) llm.Tool {
	if required == nil {
		required = []string{}
	}
	schema, _ := json.Marshal(map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false})
	return llm.Tool{Type: "function", Function: llm.Function{Name: name, Description: desc, Parameters: schema}}
}
func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func (s *Service) catalog(bot string) (map[string]binding, error) {
	var active int64
	if e := s.DB.Table("bots").Where("id = ? AND status = ?", bot, "active").Count(&active).Error; e != nil || active != 1 {
		return nil, errors.New("Bot 已暂停或不存在")
	}
	out := map[string]binding{}
	for _, t := range []llm.Tool{
		tool("workspace_create", "在 Soren 中保存一份新的 Markdown 成果。不会覆盖已有文件。", map[string]any{"title": str("文件标题"), "content": str("完整 Markdown 正文")}, "title", "content"),
		tool("workspace_list", "列出当前 Bot 的本地成果", map[string]any{}),
		tool("workspace_read", "读取当前 Bot 的本地成果", map[string]any{"id": str("成果 ID")}, "id"),
	} {
		out[t.Function.Name] = binding{Action: t.Function.Name, Tool: t}
	}
	var connections []Connection
	if e := s.DB.Where("enabled = ?", true).Order("created_at ASC").Find(&connections).Error; e != nil {
		return nil, e
	}
	// DB is scoped to the current user workspace. Every active Bot shares its enabled connections.
	for _, c := range connections {
		prefix := "c_" + strings.ReplaceAll(c.ID, "-", "") + "_"
		add := func(action, desc string, props map[string]any, approval bool, required ...string) {
			t := tool(prefix+action, c.Name+" ("+c.Account+"): "+desc, props, required...)
			out[t.Function.Name] = binding{c, action, t, approval}
		}
		switch c.Kind {
		case "gmail":
			fields := map[string]any{"to": str("单个收件人邮箱"), "subject": str("邮件主题"), "body": str("纯文本正文")}
			add("draft", "创建 Gmail 邮件草稿", fields, false, "to", "subject", "body")
			add("send", "发送邮件。必须由用户在任务中确认完整内容后执行。", fields, true, "to", "subject", "body")
		case "notion":
			add("search", "按标题搜索 Notion 中授权的页面", map[string]any{"query": str("标题关键词")}, false, "query")
			add("read", "读取页面的第一层内容块，每次最多 100 条；有更多时返回游标。", map[string]any{"pageId": str("页面 ID"), "cursor": str("可选分页游标")}, false, "pageId")
			add("create", "在指定父页面下新建页面", map[string]any{"parentId": str("父页面 ID"), "title": str("标题"), "content": str("纯文本正文")}, true, "parentId", "title", "content")
		default:
			s.addKindTools(c, add)
		case "mcp":
			for i, t := range c.Tools {
				name := fmt.Sprintf("%st%d", prefix, i)
				out[name] = binding{c, t.Name, llm.Tool{Type: "function", Function: llm.Function{Name: name, Description: c.Name + ": " + t.Description, Parameters: t.Schema}}, true}
			}
		}
	}
	if len(out) > 64 {
		return nil, errors.New("当前 Bot 的工具超过 64 个，请停用不需要的连接")
	}
	return out, nil
}

// Loop checkpoints assistant tool calls BEFORE execution. Replaying a transcript
// looks up the same invocation; an uncertain external write is never retried.
func (s *Service) Loop(ctx context.Context, p llm.Provider, bot, run string, step int, initial []llm.Message, opts *llm.Options) (*llm.Response, error) {
	id := fmt.Sprintf("%s:%d", run, step)
	var transcript Transcript
	err := s.DB.First(&transcript, "id = ?", id).Error
	messages := initial
	if errors.Is(err, gorm.ErrRecordNotFound) {
		transcript.ID = id
	} else if err != nil {
		return nil, err
	} else if err = json.Unmarshal([]byte(transcript.Messages), &messages); err != nil {
		return nil, err
	}
	save := func() error {
		b, e := json.Marshal(messages)
		if e != nil {
			return e
		}
		transcript.Messages = string(b)
		return s.DB.Save(&transcript).Error
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !s.runActive(run) {
			return nil, errors.New("任务已停止")
		}
		if s.Authorize != nil {
			if e := s.Authorize(run); e != nil {
				return nil, e
			}
		}
		catalog, e := s.catalog(bot)
		if e != nil {
			return nil, e
		}
		if s.Policy != nil {
			for name, b := range catalog {
				confirm, _, err := s.Policy(bot, run, actionKey(b))
				if err != nil {
					delete(catalog, name)
				} else {
					b.Approval = b.Approval || confirm
					catalog[name] = b
				}
			}
		}
		// Only replay assistant responses checkpointed by this run/step. Before
		// its first model round, all assistant messages are conversation history:
		// returning them would skip the new request, and executing their calls
		// could repeat an operation from an earlier run. Rounds is persisted with
		// messages, so approval/restart recovery still replays the current round.
		last := -1
		for i := len(messages) - 1; transcript.Rounds > 0 && i >= 0; i-- {
			if messages[i].Role == "assistant" {
				last = i
				break
			}
		}
		if last >= 0 {
			calls := messages[last].ToolCalls
			if len(calls) == 0 {
				return &llm.Response{Content: messages[last].Content}, nil
			}
			for _, call := range calls {
				done := false
				for _, m := range messages[last+1:] {
					if m.ToolCallID == call.ID {
						done = true
						break
					}
				}
				if done {
					continue
				}
				if len(calls) > 8 {
					return nil, errors.New("单轮工具调用超过 8 次")
				}
				key := fmt.Sprintf("%x", sha256.Sum256([]byte(id+":"+call.ID)))
				result, e := s.invoke(ctx, catalog, bot, run, key, call)
				if errors.Is(e, ErrApproval) || errors.Is(e, ErrDenied) || errors.Is(e, ErrUncertain) {
					return nil, e
				}
				if e != nil {
					result = "工具未完成：" + e.Error()
				}
				messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, Content: result})
				if e = save(); e != nil {
					return nil, e
				}
			}
		}
		if transcript.Rounds >= 12 {
			return nil, errors.New("已达到 12 轮执行上限，请缩小任务范围")
		}
		options := *opts
		options.Tools = nil
		// Stable ordering makes requests and provider caching reproducible.
		for _, name := range sortedKeys(catalog) {
			options.Tools = append(options.Tools, catalog[name].Tool)
		}
		response, e := p.Chat(ctx, messages, &options)
		if e != nil {
			return nil, e
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		seen := map[string]bool{}
		for i := range response.ToolCalls {
			call := &response.ToolCalls[i]
			if call.ID == "" {
				call.ID = uuid.NewString()
			}
			if seen[call.ID] {
				return nil, errors.New("模型返回重复工具调用标识")
			}
			seen[call.ID] = true
		}
		messages = append(messages, llm.Message{Role: "assistant", Content: response.Content, ToolCalls: response.ToolCalls})
		transcript.Rounds++
		if e = save(); e != nil {
			return nil, e
		}
	}
}
func (s *Service) runActive(id string) bool {
	var n int64
	s.DB.Table("runs").Where("id = ? AND status = ?", id, "running").Count(&n)
	return n == 1
}
func (s *Service) invoke(ctx context.Context, catalog map[string]binding, bot, run, key string, call llm.ToolCall) (string, error) {
	s.PolicyLock.RLock()
	defer s.PolicyLock.RUnlock()
	b, ok := catalog[call.Function.Name]
	if !ok {
		return "", errors.New("工具未授权或连接已断开")
	}
	policyRevision := ""
	if s.Policy != nil {
		confirm, revision, err := s.Policy(bot, run, actionKey(b))
		if err != nil {
			return "", err
		}
		b.Approval = b.Approval || confirm
		policyRevision = revision
	}
	if len(call.Function.Arguments) > 100000 || !json.Valid([]byte(call.Function.Arguments)) {
		return "", errors.New("工具参数无效或过长")
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(b.Tool.Function.Parameters, &schema); err != nil {
		return "", errors.New("工具参数定义无效")
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return "", errors.New("无法解析工具参数定义")
	}
	var arguments any
	if err = json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil {
		return "", errors.New("工具参数不是有效 JSON")
	}
	if err = resolved.Validate(arguments); err != nil {
		return "", errors.New("工具参数不符合定义，请检查必填字段、类型和额外字段")
	}
	var inv Invocation
	e := s.DB.First(&inv, "id = ?", key).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		status := "approved"
		if b.Approval {
			status = "pending"
		}
		inv = Invocation{PolicyRevision: policyRevision, ID: key, RunID: run, BotID: bot, ConnectionID: b.Connection.ID, ConnectionName: b.Connection.Name, Account: b.Connection.Account, ConnectionVersion: b.Connection.Version, Tool: call.Function.Name, Arguments: call.Function.Arguments, Status: status}
		if e = s.DB.Create(&inv).Error; e != nil {
			return "", e
		}
	} else if e != nil {
		return "", e
	}
	if inv.Arguments != call.Function.Arguments || inv.Tool != call.Function.Name {
		return "", errors.New("工具调用内容已变更")
	}
	switch inv.Status {
	case "pending":
		return "", ErrApproval
	case "succeeded":
		return inv.Result, nil
	case "denied":
		return "", ErrDenied
	case "failed", "unknown":
		return "", ErrUncertain
	case "running":
		return "", ErrUncertain
	}
	if inv.PolicyRevision != policyRevision {
		return "", errors.New("Bot 配置或授权已变更，请重新发起任务")
	}
	if b.Connection.ID != "" {
		var current Connection
		if s.DB.First(&current, "id = ?", b.Connection.ID).Error != nil || !current.Enabled || current.Version != inv.ConnectionVersion {
			return "", errors.New("连接或授权已变更，请重新发起任务")
		}
	}
	if !s.runActive(run) {
		return "", errors.New("任务已停止")
	}
	if s.Authorize != nil {
		if e := s.Authorize(run); e != nil {
			return "", e
		}
	}
	claim := s.DB.Model(&Invocation{}).Where("id = ? AND status = ?", key, "approved").Update("status", "running")
	if claim.Error != nil {
		return "", claim.Error
	}
	if claim.RowsAffected != 1 {
		return "", errors.New("操作已被处理")
	}
	output, e := s.execute(ctx, b, bot, run, []byte(call.Function.Arguments))
	if _, builtin := KindSpec(b.Connection.Kind); e == nil && builtin {
		output, e = s.captureFiles(bot, run, output)
	}
	status := "succeeded"
	if e != nil {
		status = "unknown"
		output = "操作未确认完成，请到对应应用核实后再操作。"
	}
	if len(output) > 60000 {
		output = output[:60000] + "\n[内容已截断]"
	}
	if saveErr := s.DB.Model(&Invocation{}).Where("id = ? AND status = ?", key, "running").Updates(map[string]any{"status": status, "result": output}).Error; saveErr != nil {
		return "", saveErr
	}
	if e != nil {
		return "", ErrUncertain
	}
	return output, nil
}
func (s *Service) Decide(id, owner string, approve bool) error {
	var inv Invocation
	if s.DB.First(&inv, "id = ?", id).Error != nil {
		return errors.New("操作不存在")
	}
	var count int64
	s.DB.Table("runs").Where("id = ? AND owner_bot_id = ? AND status IN ?", inv.RunID, owner, []string{"waiting_approval", "paused"}).Count(&count)
	if count != 1 {
		return errors.New("任务不属于当前 Bot 或已结束")
	}
	status, result := "approved", ""
	if !approve {
		status = "denied"
		result = "用户拒绝了此操作，不得通过其他工具重复尝试。"
	}
	res := s.DB.Model(&Invocation{}).Where("id = ? AND status = ?", id, "pending").Updates(map[string]any{"status": status, "result": result})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("此操作已处理")
	}
	return nil
}
