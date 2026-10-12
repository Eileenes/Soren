package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AnthropicProvider Anthropic提供商
type AnthropicProvider struct {
	APIKey  string
	BaseURL string
	client  *http.Client
}

// NewAnthropicProvider 创建Anthropic提供商
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	return &AnthropicProvider{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  providerClient(),
	}
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float32            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	System      string             `json:"system,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Chat 发送聊天请求
func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, options *Options) (*Response, error) {
	// 转换消息格式
	anthropicMessages := make([]anthropicMessage, 0)
	var systemPrompt string

	for _, msg := range messages {
		if msg.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content
		} else if msg.Role == "tool" {
			anthropicMessages = append(anthropicMessages, anthropicMessage{Role: "user", Content: []map[string]any{{"type": "tool_result", "tool_use_id": msg.ToolCallID, "content": msg.Content}}})
		} else if len(msg.ToolCalls) > 0 {
			blocks := []map[string]any{}
			if msg.Content != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
			}
			for _, call := range msg.ToolCalls {
				var args any
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
					return nil, fmt.Errorf("invalid tool arguments")
				}
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": call.ID, "name": call.Function.Name, "input": args})
			}
			anthropicMessages = append(anthropicMessages, anthropicMessage{Role: msg.Role, Content: blocks})
		} else {
			anthropicMessages = append(anthropicMessages, anthropicMessage{Role: msg.Role, Content: msg.Content})
		}
	}

	reqBody := anthropicRequest{
		Model:       options.Model,
		Messages:    anthropicMessages,
		MaxTokens:   options.MaxTokens,
		Temperature: options.Temperature,
		Stream:      false,
		System:      systemPrompt,
	}

	for _, tool := range options.Tools {
		reqBody.Tools = append(reqBody.Tools, anthropicTool{Name: tool.Function.Name, Description: tool.Function.Description, InputSchema: tool.Function.Parameters})
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimSuffix(p.BaseURL, "/v1")+"/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := responseBody(resp)
	if err != nil {
		return nil, err
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	text := ""
	calls := []ToolCall{}
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
		if block.Type == "tool_use" {
			calls = append(calls, ToolCall{ID: block.ID, Type: "function", Function: Function{Name: block.Name, Arguments: string(block.Input)}})
		}
	}
	return &Response{
		Content:      text,
		ToolCalls:    calls,
		Model:        anthropicResp.Model,
		FinishReason: anthropicResp.StopReason,
		Usage: &Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// StreamChat 流式聊天（未实现）
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, options *Options, stream chan<- StreamChunk) error {
	// TODO: 实现流式响应
	return fmt.Errorf("streaming not implemented yet")
}
