package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockProvider 本地回显提供商
//
// 用途：用户尚未配置任何 API Key 时，让桌面端依然能够启动并跑通
// 完整链路（对话、记忆写入与召回）。它不会调用任何外部服务，
// 只根据最后一条用户消息生成一段说明性回复，并在回复中提示
// 当前处于"未配置模型"状态，引导用户去设置页填写 Key。
//
// 这样做的目的是可用性：首次启动的应用不应该因为缺少凭据而崩溃。
type MockProvider struct{}

// NewMockProvider 创建本地回显提供商
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Chat 返回一段回显式回复
func (p *MockProvider) Chat(ctx context.Context, messages []Message, options *Options) (*Response, error) {
	// 模拟一点点思考耗时，让前端的加载态可见
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(120 * time.Millisecond):
	}

	lastUser := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUser = strings.TrimSpace(messages[i].Content)
			break
		}
	}

	// 统计本轮注入的记忆条数，方便用户直观看到记忆系统在工作
	memoryLines := 0
	for _, m := range messages {
		if m.Role == "system" && strings.Contains(m.Content, "记忆") {
			memoryLines = strings.Count(m.Content, "\n- ")
		}
	}

	var b strings.Builder
	b.WriteString("我收到你的消息了：")
	if lastUser == "" {
		b.WriteString("（空消息）")
	} else {
		b.WriteString(lastUser)
	}
	b.WriteString("\n\n")
	b.WriteString("**当前处于本地回显模式**，还没有配置大模型 API Key，所以我只能重复你的话，无法真正思考。\n\n")
	b.WriteString("在桌面应用的「设置 → 模型与连接」中，选择服务商，填写接口地址、API Key 和模型名称。\n\n")
	b.WriteString("点击「保存模型连接」后，本地服务会自动重启。然后发一条消息，即可验证模型是否连接成功。\n")

	if memoryLines > 0 {
		b.WriteString(fmt.Sprintf("\n> 顺便说一句：这一轮我召回了 %d 条长期记忆并注入了上下文，记忆系统本身已经在正常工作。", memoryLines))
	}

	content := b.String()
	return &Response{
		Content:      content,
		Model:        "soren-local-echo",
		FinishReason: "stop",
		Usage: &Usage{
			PromptTokens:     estimateTokens(messages),
			CompletionTokens: len([]rune(content)) / 2,
			TotalTokens:      estimateTokens(messages) + len([]rune(content))/2,
		},
	}, nil
}

// StreamChat 以分片形式返回与 Chat 相同的内容
func (p *MockProvider) StreamChat(ctx context.Context, messages []Message, options *Options, stream chan<- StreamChunk) error {
	resp, err := p.Chat(ctx, messages, options)
	if err != nil {
		return err
	}

	// 逐段推送，模拟流式输出
	runes := []rune(resp.Content)
	const chunkSize = 8
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case stream <- StreamChunk{Content: string(runes[i:end])}:
		}
	}

	stream <- StreamChunk{Done: true}
	return nil
}

// estimateTokens 粗略估算 token 数（中文按 1 字 ≈ 1 token 估算）
func estimateTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += len([]rune(m.Content))
	}
	return total
}
