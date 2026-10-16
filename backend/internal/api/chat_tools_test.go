package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/botconfig"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/group"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"github.com/soren/backend/internal/store"
)

type chatToolsProvider struct {
	mu           sync.Mutex
	action, args string
	names        []string
}

func (p *chatToolsProvider) Chat(_ context.Context, m []llm.Message, o *llm.Options) (*llm.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, msg := range m {
		if msg.Role == "tool" {
			return &llm.Response{Content: "已调用应用：" + msg.Content}, nil
		}
	}
	p.names = nil
	for _, tool := range o.Tools {
		p.names = append(p.names, tool.Function.Name)
	}
	for _, tool := range o.Tools {
		if strings.HasSuffix(tool.Function.Name, "_"+p.action) {
			return &llm.Response{ToolCalls: []llm.ToolCall{{ID: "call-1", Type: "function", Function: llm.Function{Name: tool.Function.Name, Arguments: p.args}}}}, nil
		}
	}
	return &llm.Response{Content: "没有获得该应用授权"}, nil
}
func (p *chatToolsProvider) StreamChat(context.Context, []llm.Message, *llm.Options, chan<- llm.StreamChunk) error {
	return nil
}

type chatFixture struct {
	r     *gin.Engine
	s     *agent.Service
	tools *connectors.Service
	p     *chatToolsProvider
	calls atomic.Int32
}

func newChatToolsFixture(t *testing.T, action, args string) *chatFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, e := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "chat.db")})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&models.Conversation{}, &models.Message{}); e != nil {
		t.Fatal(e)
	}
	f := &chatFixture{p: &chatToolsProvider{action: action, args: args}}
	f.s, e = agent.New(db, f.p, nil, "test", false)
	if e != nil {
		t.Fatal(e)
	}
	f.tools, e = connectors.New(db, &connectors.MemoryVault{}, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong credential")
		}
		if r.Method == "POST" {
			f.calls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"title":"真实测试音乐"}],"status":"queued"}`)
	}))
	t.Cleanup(upstream.Close)
	f.tools.APIBase = map[string]string{"music": upstream.URL}
	f.s.AttachTools(f.tools)
	f.r = gin.New()
	g := f.r.Group("/api")
	g.Use(BotContext(f.s))
	RegisterAgents(g, f.s)
	RegisterConnectors(g, f.tools, "")
	h := NewChatHandler(repository.NewConversationRepository(db), f.p, nil).WithAgents(f.s)
	g.POST("/chat", h.SendMessage)
	g.GET("/conversations/:id", h.GetConversation)
	g.DELETE("/conversations/:id", h.DeleteConversation)
	return f
}
func (f *chatFixture) req(t *testing.T, method, path, bot string, body any, want int) []byte {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-ID", bot)
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	if w.Code != want {
		t.Fatalf("%s %s: %d want %d %s", method, path, w.Code, want, w.Body.String())
	}
	return w.Body.Bytes()
}
func (f *chatFixture) connect(t *testing.T, allow []string) connectors.Connection {
	raw := f.req(t, "POST", "/api/connections", "soren", map[string]any{"kind": "music", "name": "AI 音乐", "token": "test-key", "allowedBots": allow}, 201)
	var c connectors.Connection
	if e := json.Unmarshal(raw, &c); e != nil {
		t.Fatal(e)
	}
	return c
}
func (f *chatFixture) start(t *testing.T, requestID, conv string) ChatResponse {
	raw := f.req(t, "POST", "/api/chat", "soren", map[string]any{"message": "使用音乐应用", "requestId": requestID, "toolExecution": true, "conversationId": conv}, 202)
	var out ChatResponse
	if e := json.Unmarshal(raw, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func (f *chatFixture) wait(t *testing.T, id, status string) agent.Run {
	t.Helper()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		var r agent.Run
		if e := json.Unmarshal(f.req(t, "GET", "/api/runs/"+id, "soren", nil, 200), &r); e != nil {
			t.Fatal(e)
		}
		if r.Status == status {
			return r
		}
		if r.Status == "failed" {
			t.Fatalf("run failed: %s", r.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run did not reach %s", status)
	return agent.Run{}
}
func (f *chatFixture) pending(t *testing.T, id string) connectors.Invocation {
	var list []connectors.Invocation
	_ = json.Unmarshal(f.req(t, "GET", "/api/runs/"+id+"/tools", "soren", nil, 200), &list)
	if len(list) != 1 || list[0].Status != "pending" {
		t.Fatalf("pending calls: %+v", list)
	}
	return list[0]
}
func (f *chatFixture) resume(t *testing.T, id string) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		e := f.s.Resume(id, "soren")
		if e == nil {
			return
		}
		if !strings.Contains(e.Error(), "仍在退出") {
			t.Fatal(e)
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("resume stayed busy")
}
func TestChatConnectedMusicExecutesAndReplayDoesNotRepeat(t *testing.T) {
	f := newChatToolsFixture(t, "list_generations", `{}`)
	f.connect(t, []string{"soren"})
	id := uuid.NewString()
	out := f.start(t, id, "")
	run := f.wait(t, out.Message.ToolRunID, "succeeded")
	if !strings.Contains(run.Result, "真实测试音乐") {
		t.Fatal(run.Result)
	}
	raw := f.req(t, "POST", "/api/chat", "soren", map[string]any{"message": "使用音乐应用", "requestId": id, "toolExecution": true}, 200)
	var replay ChatResponse
	_ = json.Unmarshal(raw, &replay)
	if replay.Message.ToolRunID != run.ID || replay.Message.Content != run.Result {
		t.Fatal("replay lost original result")
	}
	var c models.Conversation
	_ = json.Unmarshal(f.req(t, "GET", "/api/conversations/"+out.ConversationID, "soren", nil, 200), &c)
	if len(c.Messages) != 2 {
		t.Fatalf("duplicate messages: %d", len(c.Messages))
	}
}
func TestChatApprovalResumeAndRevocation(t *testing.T) {
	for _, decision := range []string{"approve", "deny", "revoke"} {
		t.Run(decision, func(t *testing.T) {
			f := newChatToolsFixture(t, "generate_music", `{"creation_id":"11111111-1111-1111-1111-111111111111","message_id":"22222222-2222-2222-2222-222222222222"}`)
			c := f.connect(t, []string{"soren"})
			out := f.start(t, uuid.NewString(), "")
			f.wait(t, out.Message.ToolRunID, "waiting_approval")
			inv := f.pending(t, out.Message.ToolRunID)
			if f.calls.Load() != 0 {
				t.Fatal("paid request before approval")
			}
			if decision == "revoke" {
				f.req(t, "PUT", "/api/connections/"+c.ID, "soren", map[string]any{"allowedBots": []string{}, "enabled": false, "version": c.Version}, 204)
			}
			f.req(t, "POST", "/api/tool-invocations/"+inv.ID+"/decision", "soren", map[string]bool{"approve": decision != "deny"}, 204)
			f.resume(t, out.Message.ToolRunID)
			if decision == "approve" {
				f.wait(t, out.Message.ToolRunID, "succeeded")
				if f.calls.Load() != 1 {
					t.Fatal("wrong number of writes")
				}
			} else {
				until := time.Now().Add(3 * time.Second)
				for time.Now().Before(until) {
					var r agent.Run
					f.s.DB.First(&r, "id = ?", out.Message.ToolRunID)
					if r.Status == "failed" || r.Status == "succeeded" {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if f.calls.Load() != 0 {
					t.Fatal("revoked or denied operation executed")
				}
			}
		})
	}
}
func TestChatWorkspaceAppStillScopesExecutionRecordsToBot(t *testing.T) {
	f := newChatToolsFixture(t, "list_generations", `{}`)
	f.connect(t, []string{})
	b := agent.Bot{Name: "Other", Avatar: "soren", Instructions: "test", Status: "active"}
	if e := f.s.SaveBot(&b, true); e != nil {
		t.Fatal(e)
	}
	out := f.start(t, uuid.NewString(), "")
	r := f.wait(t, out.Message.ToolRunID, "succeeded")
	if !strings.Contains(r.Result, "真实测试音乐") {
		t.Fatal(r.Result)
	}
	f.req(t, "GET", "/api/runs/"+r.ID, b.ID, nil, 404)
	f.req(t, "GET", "/api/runs/"+r.ID+"/tools", b.ID, nil, 404)
	f.req(t, "POST", "/api/runs/"+r.ID+"/resume", b.ID, nil, 409)
}

func TestChatCancellationBeforeArrivalAndWhileAwaitingApproval(t *testing.T) {
	f := newChatToolsFixture(t, "generate_music", `{"creation_id":"11111111-1111-1111-1111-111111111111","message_id":"22222222-2222-2222-2222-222222222222"}`)
	f.connect(t, []string{"soren"})
	id := uuid.NewString()
	f.req(t, "POST", "/api/chat/requests/"+id+"/cancel", "soren", nil, 204)
	f.req(t, "POST", "/api/chat", "soren", map[string]any{"message": "使用音乐应用", "requestId": id, "toolExecution": true}, 409)
	out := f.start(t, uuid.NewString(), "")
	f.wait(t, out.Message.ToolRunID, "waiting_approval")
	f.req(t, "POST", "/api/runs/"+out.Message.ToolRunID+"/cancel", "soren", nil, 204)
	f.wait(t, out.Message.ToolRunID, "cancelled")
	f.req(t, "POST", "/api/runs/"+out.Message.ToolRunID+"/resume", "soren", nil, 409)
	if f.calls.Load() != 0 {
		t.Fatal("cancelled action executed")
	}
}

type chatMCPTransport struct{ target *url.URL }

func (r chatMCPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *req.URL
	u.Scheme = r.target.Scheme
	u.Host = r.target.Host
	clone.URL = &u
	clone.Host = r.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}
func TestChatMCPDiscoveryApprovalAndExecution(t *testing.T) {
	f := newChatToolsFixture(t, "t0", `{"query":"test"}`)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-octobus", Version: "1"}, nil)
	var calls atomic.Int32
	mcp.AddTool(server, &mcp.Tool{Name: "search_docs", Description: "search documents"}, func(ctx context.Context, req *mcp.CallToolRequest, in struct {
		Query string `json:"query"`
	}) (*mcp.CallToolResult, map[string]string, error) {
		calls.Add(1)
		return nil, map[string]string{"result": "OctoBus test result"}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mcp-test-key" {
			t.Error("missing MCP credential")
		}
		handler.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	f.tools.MCPTransport = chatMCPTransport{target}
	f.req(t, "POST", "/api/connections", "soren", map[string]any{"kind": "mcp", "name": "OctoBus", "endpoint": "https://mcp.example.com/mcp", "token": "mcp-test-key", "allowedBots": []string{"soren"}}, 201)
	out := f.start(t, uuid.NewString(), "")
	f.wait(t, out.Message.ToolRunID, "waiting_approval")
	inv := f.pending(t, out.Message.ToolRunID)
	if calls.Load() != 0 {
		t.Fatal("MCP executed without confirmation")
	}
	f.req(t, "POST", "/api/tool-invocations/"+inv.ID+"/decision", "soren", map[string]bool{"approve": true}, 204)
	f.resume(t, out.Message.ToolRunID)
	r := f.wait(t, out.Message.ToolRunID, "succeeded")
	if calls.Load() != 1 || !strings.Contains(r.Result, "OctoBus test result") {
		t.Fatalf("MCP result %d: %s", calls.Load(), r.Result)
	}
}

func TestGroupChatToolsAvailableToEverySpeaker(t *testing.T) {
	f := newChatToolsFixture(t, "list_generations", `{}`)
	if err := f.s.DB.AutoMigrate(&models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}); err != nil {
		t.Fatal(err)
	}
	b := agent.Bot{Name: "Other", Avatar: "soren", Instructions: "test", Status: "active"}
	if err := f.s.SaveBot(&b, true); err != nil {
		t.Fatal(err)
	}
	f.connect(t, []string{"soren"})
	cr := repository.NewConversationRepository(f.s.DB)
	gr := repository.NewGroupRepository(f.s.DB)
	orch := group.New(f.s, cr, gr, f.p, nil)
	RegisterGroups(f.r, NewGroupHandler(orch, gr, cr, f.s))
	var view GroupView
	json.Unmarshal(f.req(t, "POST", "/api/groups", "", map[string]any{"title": "Tool group", "botIds": []string{"soren", b.ID}, "mode": "mention", "maxRounds": 1}, 201), &view)
	gid, _ := uuid.Parse(view.ID)
	res, err := orch.Speak(context.Background(), group.SpeakRequest{GroupID: gid, Content: "查询音乐", Mentions: []string{"soren", b.ID}, Mode: "mention", RequestID: uuid.NewString(), ToolExecution: true})
	if err != nil {
		t.Fatal(err)
	}
	replies := map[string]string{}
	for _, msg := range res.Messages {
		if msg.Role == "assistant" {
			replies[msg.SpeakerBotID] = msg.Content
			if msg.ToolRunID == "" {
				t.Fatal("missing run link")
			}
		}
	}
	if !strings.Contains(replies["soren"], "真实测试音乐") || !strings.Contains(replies[b.ID], "真实测试音乐") {
		t.Fatalf("group scope: %+v", replies)
	}
}

func TestGroupApprovalPausesAndRemovingMemberInvalidatesResume(t *testing.T) {
	f := newChatToolsFixture(t, "generate_music", `{"creation_id":"11111111-1111-1111-1111-111111111111","message_id":"22222222-2222-2222-2222-222222222222"}`)
	f.s.DB.AutoMigrate(&models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{})
	b := agent.Bot{Name: "Other", Avatar: "soren", Instructions: "test", Status: "active"}
	if e := f.s.SaveBot(&b, true); e != nil {
		t.Fatal(e)
	}
	f.connect(t, []string{"soren"})
	cr := repository.NewConversationRepository(f.s.DB)
	gr := repository.NewGroupRepository(f.s.DB)
	o := group.New(f.s, cr, gr, f.p, nil)
	RegisterGroups(f.r, NewGroupHandler(o, gr, cr, f.s))
	var v GroupView
	json.Unmarshal(f.req(t, "POST", "/api/groups", "", map[string]any{"title": "Confirm", "botIds": []string{"soren", b.ID}, "mode": "mention", "maxRounds": 1}, 201), &v)
	gid, _ := uuid.Parse(v.ID)
	result, e := o.Speak(context.Background(), group.SpeakRequest{GroupID: gid, Content: "生成音乐", Mentions: []string{"soren"}, Mode: "mention", RequestID: uuid.NewString(), ToolExecution: true})
	if e != nil || result.Status != "interrupted" {
		t.Fatalf("expected paused group: %v %+v", e, result)
	}
	var runID string
	for _, msg := range result.Messages {
		if msg.ToolRunID != "" {
			runID = msg.ToolRunID
		}
	}
	inv := f.pending(t, runID)
	f.s.DB.Where("group_id = ? AND bot_id = ?", gid, "soren").Delete(&models.GroupMember{})
	f.req(t, "POST", "/api/tool-invocations/"+inv.ID+"/decision", "soren", map[string]bool{"approve": true}, 204)
	f.req(t, "POST", "/api/runs/"+runID+"/resume", "soren", nil, 409)
	if f.calls.Load() != 0 {
		t.Fatal("removed member executed")
	}
}

func TestLegacyConnectionsWorkForNewBotsWithoutAssignment(t *testing.T) {
	for _, legacy := range [][]string{nil, {}, {"soren"}} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			f := newChatToolsFixture(t, "list_generations", `{}`)
			c := f.connect(t, nil)
			c.AllowedBots = legacy
			if err := f.s.DB.Model(&c).Select("allowed_bots").Updates(&c).Error; err != nil {
				t.Fatal(err)
			}
			b := agent.Bot{Name: "New Bot", Avatar: "soren", Instructions: "test", Status: "active"}
			if err := f.s.SaveBot(&b, true); err != nil {
				t.Fatal(err)
			}
			var out ChatResponse
			json.Unmarshal(f.req(t, "POST", "/api/chat", b.ID, map[string]any{"message": "使用音乐应用", "requestId": uuid.NewString(), "toolExecution": true}, 202), &out)
			until := time.Now().Add(5 * time.Second)
			var run agent.Run
			for time.Now().Before(until) {
				json.Unmarshal(f.req(t, "GET", "/api/runs/"+out.Message.ToolRunID, b.ID, nil, 200), &run)
				if run.Status == "succeeded" || run.Status == "failed" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if run.Status != "succeeded" || !strings.Contains(run.Result, "真实测试音乐") {
				t.Fatalf("new bot cannot use legacy connection: %+v", run)
			}
			// Availability is shared; approval and execution records still belong to their Bot.
			f.req(t, "GET", "/api/runs/"+run.ID, "soren", nil, 404)
		})
	}
}

func TestWorkspaceConnectionStillHonorsBotActionDenial(t *testing.T) {
	f := newChatToolsFixture(t, "generate_music", `{"creation_id":"11111111-1111-1111-1111-111111111111","message_id":"22222222-2222-2222-2222-222222222222"}`)
	f.connect(t, nil)
	bot, err := f.s.GetBot("soren")
	if err != nil {
		t.Fatal(err)
	}
	bot.Policy = &botconfig.Policy{Actions: map[string]string{"music.generate_music": "deny"}}
	if err = f.s.SaveBot(bot, false); err != nil {
		t.Fatal(err)
	}
	out := f.start(t, uuid.NewString(), "")
	f.wait(t, out.Message.ToolRunID, "succeeded")
	var pending int64
	f.s.DB.Model(&connectors.Invocation{}).Where("run_id = ? AND status = ?", out.Message.ToolRunID, "pending").Count(&pending)
	if pending != 0 || f.calls.Load() != 0 {
		t.Fatal("Bot action denial bypassed")
	}
}
