package templateserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/bottemplate"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOwnershipValidationAndPreviewEscaping(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "service.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	defer conn.Close()
	alice, bob := strings.Repeat("a", 43), strings.Repeat("b", 43)
	publishers := []Publisher{{"alice", "Alice", fmt.Sprintf("%x", sha256.Sum256([]byte(alice)))}, {"bob", "Bob", fmt.Sprintf("%x", sha256.Sum256([]byte(bob)))}}
	s, e := New(db, "https://templates.example", publishers)
	if e != nil {
		t.Fatal(e)
	}
	h := s.Handler()
	call := func(method, path, token string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	config := bottemplate.Config{Format: bottemplate.Format, SchemaVersion: 1, Name: "<script>alert(1)</script>", Instructions: "<img src=x onerror=alert(1)>"}
	req := bottemplate.PublishRequest{RequestID: uuid.NewString(), Config: config}
	if w := call("POST", "/api/templates", "", req); w.Code != 401 {
		t.Fatal("anonymous publication", w.Code)
	}
	w := call("POST", "/api/templates", alice, req)
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var published bottemplate.Published
	_ = json.Unmarshal(w.Body.Bytes(), &published)
	for _, method := range []string{"PATCH", "DELETE"} {
		w = call(method, "/api/templates/"+published.ID, bob, map[string]bool{"disabled": true})
		if w.Code != 404 {
			t.Fatal("other user managed template", method, w.Code)
		}
	}
	req.RequestID = uuid.NewString()
	w = call("POST", "/api/templates/"+published.ID+"/versions", bob, req)
	if w.Code != 404 {
		t.Fatal("other user published revision")
	}
	w = call("GET", "/t/"+published.ID+"/v/1", "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "<script>alert") || strings.Contains(w.Body.String(), "<img src=x") || !strings.Contains(w.Body.String(), "soren://templates/import?") {
		t.Fatal("unsafe or broken preview", w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing preview protections")
	}
	if !strings.Contains(w.Body.String(), `src="/assets/avatars/soren.svg"`) || !strings.Contains(w.Body.String(), `src="/assets/import.js"`) || !strings.Contains(w.Body.String(), published.URL) {
		t.Fatal("preview card is missing its avatar, link or app launcher")
	}
	if strings.Contains(w.Body.String(), "&lt;img src=x") {
		t.Fatal("preview should show a card, not the full Bot instructions")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self';") {
		t.Fatal("launcher must be restricted to the service's own static script")
	}
	for name := range avatarFiles {
		asset := call("GET", "/assets/avatars/"+name, "", nil)
		if asset.Code != 200 || !strings.HasPrefix(asset.Header().Get("Content-Type"), "image/") || asset.Body.Len() == 0 {
			t.Fatal("missing embedded avatar", name)
		}
	}
	if asset := call("GET", "/assets/avatars/unknown.png", "", nil); asset.Code != 404 {
		t.Fatal("unexpected asset was served")
	}
	if asset := call("GET", "/assets/import.js", "", nil); asset.Code != 200 || asset.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatal("missing app launcher")
	}
	bad := map[string]any{"requestId": uuid.NewString(), "config": map[string]any{"format": bottemplate.Format, "schemaVersion": 1, "name": "test", "instructions": "test", "apiKey": "secret"}}
	if w = call("POST", "/api/templates", alice, bad); w.Code != 400 {
		t.Fatal("unknown secret field accepted")
	}
	config.SchemaVersion = 99
	req.Config = config
	if w = call("POST", "/api/templates", alice, req); w.Code != 400 {
		t.Fatal("unknown version accepted")
	}
	r := httptest.NewRequest(http.MethodPost, "/api/templates", strings.NewReader(strings.Repeat("x", bottemplate.MaxBytes+1)))
	r.Header.Set("Authorization", "Bearer "+alice)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("oversized request accepted")
	}
}
