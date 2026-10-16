package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
)

func TestConnectionsValidateScopeHideSecretsAndDetectConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, e := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "api.db")})
	if e != nil {
		t.Fatal(e)
	}
	agents, e := agent.New(db, llm.NewMockProvider(), nil, "test", true)
	if e != nil {
		t.Fatal(e)
	}
	vault := &connectors.MemoryVault{}
	s, e := connectors.New(db, vault, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("wrong credential")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"Test workspace"}`))
	}))
	defer upstream.Close()
	s.NotionAPI = upstream.URL
	r := gin.New()
	g := r.Group("/api")
	g.Use(BotContext(agents))
	RegisterConnectors(g, s, "http://127.0.0.1/callback")
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	// Older clients may still send selections; the server ignores them.
	saved := request("POST", "/api/connections", `{"kind":"notion","token":"test-secret","allowedBots":["missing"]}`)
	if saved.Code != 201 {
		t.Fatal(saved.Code, saved.Body.String())
	}
	if strings.Contains(saved.Body.String(), "test-secret") {
		t.Fatal("credential returned")
	}
	var c connectors.Connection
	if e = json.Unmarshal(saved.Body.Bytes(), &c); e != nil {
		t.Fatal(e)
	}
	update := request("PUT", "/api/connections/"+c.ID, `{"allowedBots":[],"enabled":false,"version":1}`)
	if update.Code != 204 {
		t.Fatal(update.Code, update.Body.String())
	}
	stale := request("PUT", "/api/connections/"+c.ID, `{"allowedBots":["soren"],"enabled":true,"version":1}`)
	if stale.Code != 409 {
		t.Fatal("stale permissions overwrote newer state")
	}
	list := request("GET", "/api/connections", "")
	if strings.Contains(list.Body.String(), "test-secret") {
		t.Fatal("credential leaked in list")
	}
	var rows []connectors.Connection
	_ = json.Unmarshal(list.Body.Bytes(), &rows)
	if len(rows) != 1 || rows[0].Enabled || len(rows[0].AllowedBots) != 0 || rows[0].Version != 2 {
		t.Fatal(rows)
	}
	gone := request("DELETE", "/api/connections/"+c.ID, "")
	if gone.Code != 204 {
		t.Fatal(gone.Code)
	}
	if _, e = vault.Get(c.ID); e == nil {
		t.Fatal("credential not removed")
	}
}
