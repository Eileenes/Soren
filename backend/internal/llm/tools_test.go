package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIToolRoundTrip(t *testing.T) {
	var request openAIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
			t.Error(e)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`))
	}))
	defer srv.Close()
	p := NewOpenAIProvider("test", srv.URL)
	res, e := p.Chat(context.Background(), []Message{{Role: "user", Content: "read"}}, &Options{Model: "test", Tools: []Tool{{Type: "function", Function: Function{Name: "read", Parameters: json.RawMessage(`{"type":"object"}`)}}}})
	if e != nil || len(res.ToolCalls) != 1 || res.ToolCalls[0].ID != "c1" || len(request.Tools) != 1 {
		t.Fatal(res, e, request)
	}
}
func TestAnthropicToolBlocksAndResults(t *testing.T) {
	var request map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&request)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"checking"},{"type":"tool_use","id":"c2","name":"read","input":{"id":"doc"}}],"stop_reason":"tool_use"}`))
	}))
	defer srv.Close()
	p := NewAnthropicProvider("test", srv.URL)
	res, e := p.Chat(context.Background(), []Message{
		{Role: "user", Content: "read"},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "c1", Function: Function{Name: "read", Arguments: `{"id":"doc"}`}}}},
		{Role: "tool", ToolCallID: "c1", Content: "result"},
	}, &Options{Model: "test", MaxTokens: 100, Tools: []Tool{{Function: Function{Name: "read", Parameters: json.RawMessage(`{"type":"object"}`)}}}})
	if e != nil || len(res.ToolCalls) != 1 || res.Content != "checking" {
		t.Fatal(res, e)
	}
	messages := request["messages"].([]any)
	block := messages[2].(map[string]any)["content"].([]any)[0].(map[string]any)
	if block["type"] != "tool_result" || block["tool_use_id"] != "c1" || block["content"] != "result" {
		t.Fatal(block)
	}
}
