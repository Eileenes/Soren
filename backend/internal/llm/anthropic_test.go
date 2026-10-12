package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicPreservesRoleAndMemorySystemMessages(t *testing.T) {
	var body anthropicRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"ok"}],"model":"test"}`))
	}))
	defer server.Close()
	p := NewAnthropicProvider("test", server.URL)
	_, err := p.Chat(context.Background(), []Message{{Role: "system", Content: "ROLE_SPEC"}, {Role: "system", Content: "PRIVATE_MEMORY"}, {Role: "user", Content: "hello"}}, &Options{Model: "test", MaxTokens: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.System, "ROLE_SPEC") || !strings.Contains(body.System, "PRIVATE_MEMORY") {
		t.Fatal("system messages overwritten")
	}
	if len(body.Messages) != 1 {
		t.Fatal("system was passed as a conversation role")
	}
}
