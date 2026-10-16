package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/api"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type workspaceToolsProvider struct{}

func (workspaceToolsProvider) Chat(_ context.Context, m []llm.Message, o *llm.Options) (*llm.Response, error) {
	for _, msg := range m {
		if msg.Role == "user" && msg.Content == "list connected apps" {
			for _, tool := range o.Tools {
				if strings.HasPrefix(tool.Function.Name, "c_") {
					return &llm.Response{Content: "apps-visible"}, nil
				}
			}
			return &llm.Response{Content: "apps-none"}, nil
		}

		if msg.Role == "tool" {
			return &llm.Response{Content: "saved"}, nil
		}
	}
	for _, tool := range o.Tools {
		if tool.Function.Name == "workspace_create" {
			return &llm.Response{ToolCalls: []llm.ToolCall{{ID: "create", Type: "function", Function: llm.Function{Name: "workspace_create", Arguments: `{"title":"private","content":"alice private artifact"}`}}}}, nil
		}
	}
	return &llm.Response{Content: "plain reply"}, nil
}
func (workspaceToolsProvider) StreamChat(context.Context, []llm.Message, *llm.Options, chan<- llm.StreamChunk) error {
	return nil
}
func TestPostgresChatToolsRemainInUserWorkspace(t *testing.T) {
	dsn := os.Getenv("SOREN_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("SOREN_TEST_POSTGRES_URL not configured")
	}
	db, e := store.OpenDB(&config.Config{StorageDriver: "postgres", DatabaseURL: dsn})
	if e != nil {
		t.Fatal(e)
	}
	sql, _ := db.DB()
	defer sql.Close()
	c := Config{DatabaseURL: dsn, DataDir: t.TempDir(), Key: bytes.Repeat([]byte{1}, 32), Concurrency: 2}
	ws := NewWorkspaces(c, db, workspaceToolsProvider{}, "test", "test")
	ws.Active = func(string) error { return nil }
	var handlers []http.Handler
	var schemas []string
	for i := 0; i < 2; i++ {
		id := uuid.NewString()
		schema := "workspace_" + strings.ReplaceAll(id, "-", "")
		schemas = append(schemas, schema)
		defer db.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
		h, e := ws.Handler(User{ID: id})
		if e != nil {
			t.Fatal(e)
		}
		handlers = append(handlers, h)
	}
	call := func(who int, method, path string, body any, want int) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Bot-ID", "soren")
		w := httptest.NewRecorder()
		handlers[who].ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	requestID := uuid.NewString()
	input := map[string]any{"message": "create artifact", "requestId": requestID, "toolExecution": true}
	var out api.ChatResponse
	json.Unmarshal(call(0, "POST", "/api/chat", input, 202), &out)
	var run agent.Run
	until := time.Now().Add(8 * time.Second)
	for time.Now().Before(until) {
		json.Unmarshal(call(0, "GET", "/api/runs/"+out.Message.ToolRunID, nil, 200), &run)
		if run.Status == "succeeded" || run.Status == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if run.Status != "succeeded" {
		t.Fatalf("chat run: %s %s", run.Status, run.Error)
	}
	if !bytes.Contains(call(0, "GET", "/api/runs/"+run.ID+"/artifacts", nil, 200), []byte("alice private artifact")) {
		t.Fatal("artifact missing")
	}
	for _, suffix := range []string{"", "/tools", "/artifacts", "/files"} {
		call(1, "GET", "/api/runs/"+run.ID+suffix, nil, 404)
	}
	call(1, "POST", "/api/runs/"+run.ID+"/resume", nil, 409)
	call(0, "POST", "/api/chat", input, 200)
	var messages struct {
		Messages []any `json:"messages"`
	}
	json.Unmarshal(call(0, "GET", "/api/conversations/"+out.ConversationID, nil, 200), &messages)
	if len(messages.Messages) != 2 {
		t.Fatal("replay duplicated messages")
	}
	// A legacy connection belongs only to Alice's schema, regardless of Bot selection.
	conn := connectors.Connection{ID: uuid.NewString(), Kind: "music", Name: "Alice music", Enabled: true, Version: 1, AllowedBots: []string{"deleted-bot"}, Tools: []connectors.RemoteTool{}}
	if err := db.Table(schemas[0] + ".connections").Create(&conn).Error; err != nil {
		t.Fatal(err)
	}
	for who, want := range []string{"apps-visible", "apps-none"} {
		var response api.ChatResponse
		json.Unmarshal(call(who, "POST", "/api/chat", map[string]any{"message": "list connected apps", "requestId": uuid.NewString(), "toolExecution": true}, 202), &response)
		var result agent.Run
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			json.Unmarshal(call(who, "GET", "/api/runs/"+response.Message.ToolRunID, nil, 200), &result)
			if result.Status == "succeeded" || result.Status == "failed" {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if result.Status != "succeeded" || result.Result != want {
			t.Fatalf("workspace %d visibility: %+v", who, result)
		}
	}

}
