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

// Every Baizhi kind validates the key against a free endpoint before saving it.
func TestBaizhiConnectionsValidateKeyBeforeSaving(t *testing.T) {
	for kind, check := range map[string]string{"music": "/generations", "image": "/pricing"} {
		t.Run(kind, func(t *testing.T) {
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
				if r.Method != "GET" || r.URL.Path != check {
					t.Errorf("validation hit %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer good-key" {
					w.WriteHeader(401)
					return
				}
				w.Write([]byte(`{}`))
			}))
			defer upstream.Close()
			s.APIBase = map[string]string{kind: upstream.URL}
			r := gin.New()
			g := r.Group("/api")
			g.Use(BotContext(agents))
			RegisterConnectors(g, s, "http://127.0.0.1/callback")
			post := func(body string) *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", "/api/connections", bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				return w
			}
			if w := post(`{"kind":"` + kind + `","token":"bad-key","allowedBots":["soren"]}`); w.Code != 400 {
				t.Fatal("bad key saved", w.Code)
			}
			if w := post(`{"kind":"` + kind + `","token":"","allowedBots":["soren"]}`); w.Code != 400 {
				t.Fatal("empty key saved", w.Code)
			}
			w := post(`{"kind":"` + kind + `","name":"` + kind + `","token":"good-key","allowedBots":["soren"]}`)
			if w.Code != 201 || strings.Contains(w.Body.String(), "good-key") {
				t.Fatal(w.Code, w.Body.String())
			}
			var c connectors.Connection
			_ = json.Unmarshal(w.Body.Bytes(), &c)
			if cred, e := vault.Get(c.ID); e != nil || cred.APIKey != "good-key" {
				t.Fatal("key not stored in vault")
			}
			k, _ := connectors.KindSpec(kind)
			if c.Name != k.Name() || c.Kind != kind {
				t.Fatal(c)
			}
		})
	}
}
