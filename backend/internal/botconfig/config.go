// Package botconfig contains portable role definitions, never account grants.
package botconfig

import (
	"errors"
	"regexp"
	"strings"
)

type Definition struct {
	CanDo    []string `json:"canDo"`
	CannotDo []string `json:"cannotDo"`
	Workflow string   `json:"workflow"`
	Output   string   `json:"output"`
}

// A nil policy preserves existing installations. A configured policy denies all
// actions that are not explicitly listed, including newly discovered MCP tools.
type Policy struct {
	Actions map[string]string `json:"actions"`
}

func ActionLabel(key string) string {
	labels := map[string]string{"workspace_create": "保存本地成果", "workspace_list": "查看本地成果列表", "workspace_read": "读取本地成果", "gmail.draft": "创建邮件草稿", "gmail.send": "发送邮件", "notion.search": "搜索 Notion 页面", "notion.read": "读取 Notion 页面", "notion.create": "创建 Notion 页面"}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

var ActionPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,160}$`)

func Validate(d Definition, p *Policy) error {
	if len(d.CanDo) > 30 || len(d.CannotDo) > 30 || len([]rune(d.Workflow)) > 4000 || len([]rune(d.Output)) > 4000 {
		return errors.New("职责条目最多各 30 条，工作方式与输出标准最多各 4000 字")
	}
	for _, rows := range [][]string{d.CanDo, d.CannotDo} {
		for _, row := range rows {
			if strings.TrimSpace(row) == "" || len([]rune(row)) > 500 {
				return errors.New("每条职责需为 1–500 字")
			}
		}
	}
	if p != nil {
		if len(p.Actions) > 128 {
			return errors.New("动作策略最多 128 项")
		}
		for key, value := range p.Actions {
			if !ActionPattern.MatchString(key) || (value != "allow" && value != "confirm" && value != "deny") {
				return errors.New("动作策略无效")
			}
		}
	}
	return nil
}

func Prompt(instructions string, d Definition) string {
	out := instructions
	for _, section := range []struct {
		title string
		rows  []string
	}{{"负责事项", d.CanDo}, {"不处理的事项", d.CannotDo}} {
		if len(section.rows) > 0 {
			out += "\n\n" + section.title + "：\n- " + strings.Join(section.rows, "\n- ")
		}
	}
	if d.Workflow != "" {
		out += "\n\n工作方式：\n" + d.Workflow
	}
	if d.Output != "" {
		out += "\n\n输出标准：\n" + d.Output
	}
	return out
}

func Decision(p *Policy, action string) (bool, error) {
	if p == nil {
		return false, nil
	}
	switch p.Actions[action] {
	case "allow":
		return false, nil
	case "confirm":
		return true, nil
	default:
		return false, errors.New("Bot 未授权此动作：" + action)
	}
}
