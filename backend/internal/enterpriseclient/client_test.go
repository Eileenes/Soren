package enterpriseclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/store"
)

func TestSessionFailuresRemainRetryableAndLogoutReportsOutage(t *testing.T) {
	type response struct {
		status int
		body   string
	}
	var reply atomic.Value
	valid := response{200, `{"workspaceId":"alice-space","user":{"id":"alice","name":"Alice"}}`}
	reply.Store(valid)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out := reply.Load().(response)
		w.WriteHeader(out.status)
		_, _ = w.Write([]byte(out.body))
	}))
	defer upstream.Close()
	db, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "local.db")})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	vault := &connectors.MemoryVault{}
	client, err := New(db, vault, true)
	if err != nil {
		t.Fatal(err)
	}
	row := Record{ID: 1, Active: true, ServerURL: upstream.URL, WorkspaceID: "alice-space", UserID: "alice"}
	if err := db.Save(&row).Error; err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("s", 43)
	if err := vault.Set(vaultID, connectors.Credential{APIKey: token, ClientID: row.binding()}); err != nil {
		t.Fatal(err)
	}
	client.pending = &loginAttempt{ID: "attempt", Expires: time.Now().Add(time.Minute), ServerURL: upstream.URL, Token: token, Connection: row}
	router := gin.New()
	client.Register(router)
	call := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/enterprise/"+path, strings.NewReader(`{"id":"attempt"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		return rec
	}
	for _, out := range []response{{503, `{"error":"database unavailable"}`}, {200, `{}`}, {200, `invalid-json`}, {401, `{"error":"expired"}`}, {200, `{"workspaceId":"bob-space","user":{"id":"bob"}}`}} {
		reply.Store(out)
		expired := out.status == 401 || strings.Contains(out.body, "bob-space")
		result := call("GET", "status")
		var status struct {
			Mode       string `json:"mode"`
			NeedsLogin bool   `json:"needsLogin"`
			Error      string `json:"error"`
		}
		if json.Unmarshal(result.Body.Bytes(), &status) != nil || result.Code != 200 || status.Mode != "enterprise" || status.NeedsLogin != expired || (!expired && status.Error == "") {
			t.Fatalf("incorrect status for %+v: %s", out, result.Body.String())
		}
		want := 503
		if expired {
			want = 401
		}
		if result = call("POST", "complete"); result.Code != want {
			t.Fatalf("incorrect confirmation failure for %+v: %d %s", out, result.Code, result.Body.String())
		}
		if saved, err := vault.Get(vaultID); err != nil || saved.APIKey != token {
			t.Fatal("failed verification removed stored session")
		}
	}
	reply.Store(valid)
	if result := call("POST", "complete"); result.Code != 200 {
		t.Fatalf("confirmation did not recover: %s", result.Body.String())
	}
	if result := call("GET", "status"); strings.Contains(result.Body.String(), "needsLogin") || strings.Contains(result.Body.String(), `"error"`) {
		t.Fatalf("saved session did not recover: %s", result.Body.String())
	}
	reply.Store(response{503, `{"error":"database unavailable"}`})
	result := call("POST", "disconnect")
	if result.Code != 200 || !strings.Contains(result.Body.String(), `"revoked":false`) || !strings.Contains(result.Body.String(), `"credentialDeleted":true`) {
		t.Fatalf("incorrect logout status: %s", result.Body.String())
	}
}

func TestBridgeKeepsTokenInVaultAndRequiresActiveCompany(t *testing.T) {
	token := strings.Repeat("t", 43)
	poll := strings.Repeat("p", 43)
	ticket := strings.Repeat("a", 43)
	upstream := httptest.NewUnstartedServer(nil)
	root := "http://" + upstream.Listener.Addr().String()
	identity := map[string]any{"workspaceId": "alice-space", "organization": "Example", "user": map[string]string{"id": "alice", "name": "Alice"}}
	upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/soren":
			_ = json.NewEncoder(w).Encode(Discovery{Product: "soren", ProtocolVersion: 1, Name: "Example", ServerURL: root, Auth: "oauth-browser"})
		case "/auth/attempts":
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": ticket, "pollSecret": poll, "url": root + "/auth/browser?ticket=" + ticket, "expiresAt": time.Now().Add(5 * time.Minute)})
		case "/auth/poll":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["pollSecret"] != poll {
				t.Error("wrong poll secret")
			}
			out := map[string]any{"token": token}
			for k, v := range identity {
				out[k] = v
			}
			_ = json.NewEncoder(w).Encode(out)
		case "/api/session":
			if r.Header.Get("Authorization") != "Bearer "+token {
				t.Error("missing server session")
			}
			_ = json.NewEncoder(w).Encode(identity)
		case "/api/bots":
			if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-User-ID") != "" || r.Header.Get("X-Workspace-ID") != "" {
				t.Error("unsafe proxy identity")
			}
			_, _ = w.Write([]byte(`[{"id":"soren","name":"Company Bot"}]`))
		case "/api/session/logout":
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected upstream request: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	upstream.Start()
	defer upstream.Close()
	db, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "local.db")})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	v := &connectors.MemoryVault{}
	client, err := New(db, v, true)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	client.Register(router)
	call := func(method, path string, body any) (int, string) {
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "mallory")
		req.Header.Set("X-Workspace-ID", "someone-else")
		req.Header.Set("Authorization", "Bearer attacker")
		req.Header.Set("X-Soren-Workspace", root+":alice-space:alice")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), token) || strings.Contains(rec.Body.String(), poll) {
			t.Fatal("credential exposed to renderer")
		}
		return rec.Code, rec.Body.String()
	}
	if status, _ := call("GET", "/enterprise/api/bots", nil); status != 401 {
		t.Fatal("proxy before login")
	}
	if status, raw := call("POST", "/api/enterprise/login", map[string]string{"serverUrl": root}); status != 200 {
		t.Fatalf("login %d %s", status, raw)
	}
	if status, _ := call("POST", "/api/enterprise/complete", map[string]string{"id": ticket}); status != 409 {
		t.Fatal("activated without browser authorization")
	}
	if status, raw := call("POST", "/api/enterprise/poll", map[string]string{"id": ticket}); status != 200 {
		t.Fatalf("poll %d %s", status, raw)
	}
	if status, _ := call("GET", "/enterprise/api/bots", nil); status != 401 {
		t.Fatal("poll activated company before confirmation")
	}
	if _, err := v.Get(vaultID); err == nil {
		t.Fatal("poll persisted unconfirmed credentials")
	}
	if status, _ := call("POST", "/api/enterprise/complete", map[string]string{"id": "stale"}); status != 401 {
		t.Fatal("stale panel committed login")
	}
	if status, _ := call("POST", "/api/enterprise/cancel", map[string]string{"id": ticket}); status != 204 {
		t.Fatal("cancel failed")
	}
	if status, _ := call("POST", "/api/enterprise/complete", map[string]string{"id": ticket}); status != 401 {
		t.Fatal("cancelled login committed")
	}
	call("POST", "/api/enterprise/login", map[string]string{"serverUrl": root})
	call("POST", "/api/enterprise/poll", map[string]string{"id": ticket})
	if status, raw := call("POST", "/api/enterprise/complete", map[string]string{"id": ticket}); status != 200 {
		t.Fatalf("complete %d %s", status, raw)
	}
	// Requests issued for another identity must not use the newly selected session.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/enterprise/api/bots", nil))
	if rec.Code != 409 {
		t.Fatal("accepted missing workspace guard")
	}
	if status, raw := call("GET", "/enterprise/api/bots", nil); status != 200 || !strings.Contains(raw, "Company Bot") {
		t.Fatal("proxy failed")
	}
	// Simulate an interrupted account switch: new vault token, old local record.
	saved, _ := v.Get(vaultID)
	wrong := saved
	wrong.ClientID = root + ":bob-space:bob"
	_ = v.Set(vaultID, wrong)
	if status, _ := call("GET", "/enterprise/api/bots", nil); status != 401 {
		t.Fatal("used another user's token after partial save")
	}
	_ = v.Set(vaultID, saved)
	// A fresh bridge restores from the system vault and persisted non-secret metadata.
	client, err = New(db, v, true)
	if err != nil {
		t.Fatal(err)
	}
	router = gin.New()
	client.Register(router)
	if status, raw := call("GET", "/api/enterprise/status", nil); status != 200 || !strings.Contains(raw, `"mode":"enterprise"`) {
		t.Fatal("did not restore company")
	}
	if status, _ := call("GET", "/enterprise/auth/attempts", nil); status != 404 {
		t.Fatal("proxied outside business API")
	}
	if status, _ := call("POST", "/api/enterprise/disconnect", map[string]string{}); status != 200 {
		t.Fatal("disconnect failed")
	}
	if _, err = v.Get(vaultID); err == nil {
		t.Fatal("credential survived disconnect")
	}
	if status, _ := call("GET", "/enterprise/api/bots", nil); status != 401 {
		t.Fatal("company access after disconnect")
	}
}

func TestDiscoveryRejectsRedirectsAndMismatchedOrigins(t *testing.T) {
	db, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "local.db")})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	client, _ := New(db, &connectors.MemoryVault{}, true)
	untrusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://untrusted.example", 302) }))
	defer untrusted.Close()
	if _, err = client.discover(t.Context(), untrusted.URL); err == nil {
		t.Fatal("followed discovery redirect")
	}
	mismatch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Discovery{Product: "soren", ProtocolVersion: 1, Name: "wrong", ServerURL: "https://different.example", Auth: "oauth-browser"})
	}))
	defer mismatch.Close()
	if _, err = client.discover(t.Context(), mismatch.URL); err == nil {
		t.Fatal("accepted changed server origin")
	}
}
