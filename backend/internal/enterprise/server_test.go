package enterprise

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	t.Cleanup(func() { _ = sql.Close() })
	return db
}

func TestDatabaseOutageDoesNotExpireOrRevokeSession(t *testing.T) {
	for _, table := range []string{"enterprise_sessions", "enterprise_users"} {
		t.Run(table, func(t *testing.T) {
			h := newHarness(t)
			token, _ := h.login(t, "alice")
			var unavailable atomic.Bool
			if err := h.db.Callback().Query().Before("gorm:query").Register("test:database-outage", func(db *gorm.DB) {
				if unavailable.Load() && db.Statement.Table == table {
					db.AddError(errors.New("temporary database failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			unavailable.Store(true)
			for _, endpoint := range []struct{ method, path string }{{"GET", "/api/session"}, {"GET", "/api/bots"}, {"POST", "/api/session/logout"}} {
				if status, _ := h.request(t, endpoint.method, endpoint.path, token, nil, nil); status != 503 {
					t.Fatalf("database outage returned %d for %s, want 503", status, endpoint.path)
				}
			}
			unavailable.Store(false)
			if status, _ := h.request(t, "GET", "/api/session", token, nil, nil); status != 200 {
				t.Fatalf("session did not recover: %d", status)
			}
			if status, _ := h.request(t, "POST", "/api/session/logout", token, nil, nil); status != 204 {
				t.Fatalf("logout after recovery: %d", status)
			}
			if status, _ := h.request(t, "GET", "/api/session", token, nil, nil); status != 401 {
				t.Fatalf("revoked session accepted: %d", status)
			}
		})
	}
}

type harness struct {
	s         *Server
	server    *httptest.Server
	client    *http.Client
	db        *gorm.DB
	challenge string
	mu        sync.Mutex
}

func newHarness(t *testing.T, authMode ...string) *harness {
	t.Helper()
	h := &harness{db: testDB(t), client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			h.mu.Lock()
			challenge := h.challenge
			h.mu.Unlock()
			client, secret, ok := r.BasicAuth()
			if !ok || client != "test-client" || secret != "test-secret" || base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
				http.Error(w, "invalid client or pkce", 400)
				return
			}
			respond(w, 200, map[string]string{"access_token": r.Form.Get("code"), "token_type": "Bearer"})
		case "/userinfo":
			subject := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			respond(w, 200, map[string]string{"sub": subject, "name": subject + " name"})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(idp.Close)
	h.server = httptest.NewUnstartedServer(nil)
	root := "http://" + h.server.Listener.Addr().String()
	c := Config{PublicURL: root, Name: "Test Company", DatabaseURL: "postgres://test@localhost/test", DataDir: t.TempDir(), AuthorizationURL: idp.URL + "/authorize", TokenURL: idp.URL + "/token", UserInfoURL: idp.URL + "/userinfo", ClientID: "test-client", ClientSecret: "test-secret", SubjectClaim: "sub", Scope: "openid profile", AuthMethod: "client_secret_basic", AllowedSubjects: map[string]bool{"alice": true, "bob": true}, Key: bytes.Repeat([]byte{7}, 32), Development: true, SessionTTL: time.Hour, Concurrency: 2}
	if len(authMode) > 0 {
		c.AuthMode = authMode[0]
	}
	var mutex sync.Mutex
	handlers := map[string]http.Handler{}
	factory := func(u User) (http.Handler, error) {
		mutex.Lock()
		defer mutex.Unlock()
		if existing := handlers[u.ID]; existing != nil {
			return existing, nil
		}
		handler, err := WorkspaceHandler(testDB(t), t.TempDir(), c.Key, u.ID, llm.NewMockProvider(), "test", "local-echo", func() error { return h.s.Active(u.ID) })
		if err == nil {
			handlers[u.ID] = handler
		}
		return handler, err
	}
	var err error
	h.s, err = New(c, h.db, factory)
	if err != nil {
		t.Fatal(err)
	}
	h.server.Config.Handler = h.s.Handler()
	h.server.Start()
	t.Cleanup(h.server.Close)
	return h
}
func (h *harness) request(t *testing.T, method, path, token string, body any, headers map[string]string) (int, []byte) {
	t.Helper()
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, h.server.URL+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}
func (h *harness) login(t *testing.T, subject string) (string, string) {
	t.Helper()
	status, raw := h.request(t, "POST", "/auth/attempts", "", map[string]any{}, nil)
	if status != 201 {
		t.Fatalf("start: %d", status)
	}
	var start struct {
		ID     string `json:"id"`
		Secret string `json:"pollSecret"`
		URL    string `json:"url"`
	}
	_ = json.Unmarshal(raw, &start)
	status, _ = h.request(t, "POST", "/auth/poll", "", map[string]string{"id": start.ID, "pollSecret": "wrong"}, nil)
	if status != 401 {
		t.Fatal("accepted wrong poll credential")
	}
	res, err := h.client.Get(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 302 {
		t.Fatal("browser login did not redirect")
	}
	location, _ := url.Parse(res.Header.Get("Location"))
	h.mu.Lock()
	h.challenge = location.Query().Get("code_challenge")
	h.mu.Unlock()
	if location.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("missing PKCE")
	}
	callback := "/auth/callback?state=" + url.QueryEscape(location.Query().Get("state")) + "&code=" + url.QueryEscape(subject)
	status, _ = h.request(t, "GET", callback, "", nil, nil)
	if status != 400 {
		t.Fatal("accepted callback without bound browser cookie")
	}
	req, _ := http.NewRequest("GET", h.server.URL+callback, nil)
	for _, cookie := range res.Cookies() {
		req.AddCookie(cookie)
	}
	res, err = h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if subject == "outsider" {
		if res.StatusCode != 400 {
			t.Fatal("accepted unauthorized subject")
		}
		return "", ""
	}
	if res.StatusCode != 200 {
		t.Fatalf("callback: %d", res.StatusCode)
	}
	res, err = h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatal("callback replay accepted")
	}
	status, raw = h.request(t, "POST", "/auth/poll", "", map[string]string{"id": start.ID, "pollSecret": start.Secret}, nil)
	if status != 200 {
		t.Fatalf("poll: %d", status)
	}
	var result struct {
		Token       string `json:"token"`
		WorkspaceID string `json:"workspaceId"`
	}
	_ = json.Unmarshal(raw, &result)
	if len(result.Token) != 43 || result.WorkspaceID == "" {
		t.Fatal("missing session")
	}
	return result.Token, result.WorkspaceID
}

func TestOAuthWorkspaceIsolationAndRevocation(t *testing.T) {
	h := newHarness(t)
	if status, _ := h.request(t, "GET", "/api/bots", "", nil, map[string]string{"X-User-ID": "alice"}); status != 401 {
		t.Fatal("client user hint authenticated")
	}
	h.login(t, "outsider")
	alice, aliceID := h.login(t, "alice")
	bob, bobID := h.login(t, "bob")
	if aliceID == bobID {
		t.Fatal("shared workspace")
	}
	status, raw := h.request(t, "POST", "/api/bots", alice, map[string]any{"name": "Alice private bot", "avatar": "soren", "instructions": "Private instructions", "status": "active", "version": 1, "collaborators": []string{}}, nil)
	if status != 201 {
		t.Fatalf("create bot: %d %s", status, raw)
	}
	var bot struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &bot)
	status, raw = h.request(t, "GET", "/api/bots", bob, nil, map[string]string{"X-User-ID": aliceID, "X-Workspace-ID": aliceID})
	if status != 200 || bytes.Contains(raw, []byte("Alice private")) {
		t.Fatalf("bot list leak: %d", status)
	}
	status, _ = h.request(t, "GET", "/api/memories", bob, nil, map[string]string{"X-Bot-ID": bot.ID})
	if status != 404 {
		t.Fatal("foreign bot accepted")
	}
	status, raw = h.request(t, "POST", "/api/chat", alice, map[string]string{"message": "Alice private conversation"}, nil)
	if status != 200 {
		t.Fatalf("chat: %d %s", status, raw)
	}
	var chat struct {
		ConversationID string `json:"conversationId"`
	}
	_ = json.Unmarshal(raw, &chat)
	for _, method := range []string{"GET", "DELETE"} {
		status, _ = h.request(t, method, "/api/conversations/"+chat.ConversationID, bob, nil, nil)
		if status != 404 {
			t.Fatalf("foreign conversation %s accepted: %d", method, status)
		}
	}
	status, raw = h.request(t, "GET", "/api/conversations", bob, nil, nil)
	if status != 200 || bytes.Contains(raw, []byte(chat.ConversationID)) {
		t.Fatal("conversation list leak")
	}
	status, raw = h.request(t, "POST", "/api/memories", alice, map[string]string{"content": "Alice private memory marker"}, nil)
	if status != 201 {
		t.Fatalf("save memory: %d %s", status, raw)
	}
	status, raw = h.request(t, "GET", "/api/memories", bob, nil, map[string]string{"X-User-ID": aliceID})
	if status != 200 || bytes.Contains(raw, []byte("Alice private memory")) {
		t.Fatal("memory leak")
	}
	status, raw = h.request(t, "POST", "/api/shared-notes", alice, map[string]any{"title": "Alice private note", "content": "confidential", "allowedBots": []string{"soren"}}, nil)
	if status != 200 {
		t.Fatalf("create note: %d %s", status, raw)
	}
	var note struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &note)
	status, raw = h.request(t, "GET", "/api/shared-notes", bob, nil, nil)
	if status != 200 || bytes.Contains(raw, []byte("Alice private note")) {
		t.Fatal("shared note escaped employee scope")
	}
	if status, _ := h.request(t, "DELETE", "/api/shared-notes/"+note.ID, bob, nil, nil); status != 404 {
		t.Fatal("deleted another employee's note")
	}
	status, raw = h.request(t, "POST", "/api/groups", alice, map[string]any{"title": "Alice private group", "botIds": []string{"soren", bot.ID}, "mode": "mention", "maxRounds": 2}, nil)
	if status != 201 {
		t.Fatalf("create group: %d %s", status, raw)
	}
	var group struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &group)
	if status, _ := h.request(t, "GET", "/api/groups/"+group.ID, bob, nil, nil); status != 404 {
		t.Fatal("read another employee's group")
	}
	status, _ = h.request(t, "GET", "/api/session", alice, nil, map[string]string{"Origin": "https://untrusted.example"})
	if status != 403 {
		t.Fatal("cross-origin request accepted")
	}
	if err := h.db.Model(&User{}).Where("id = ?", aliceID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = h.request(t, "GET", "/api/bots", alice, nil, nil)
	if status != 401 {
		t.Fatal("disabled user still authenticated")
	}
	status, _ = h.request(t, "POST", "/api/session/logout", bob, map[string]any{}, nil)
	if status != 204 {
		t.Fatal("logout failed")
	}
	status, _ = h.request(t, "GET", "/api/bots", bob, nil, nil)
	if status != 401 {
		t.Fatal("logged-out token still works")
	}
}

func TestVaultBindsCiphertextToWorkspaceAndCredential(t *testing.T) {
	db := testDB(t)
	key := bytes.Repeat([]byte{4}, 32)
	v, err := newVault(db, key, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if err = v.Set("connection", connectors.Credential{APIKey: "private-test-token"}); err != nil {
		t.Fatal(err)
	}
	var stored credential
	if err = db.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored.Ciphertext, []byte("private-test-token")) {
		t.Fatal("plaintext credential stored")
	}
	reopened, _ := newVault(db, key, "alice")
	cred, err := reopened.Get("connection")
	if err != nil || cred.APIKey != "private-test-token" {
		t.Fatal("credential did not survive reopening")
	}
	other, _ := newVault(db, key, "bob")
	if _, err = other.Get("connection"); err == nil {
		t.Fatal("cross-workspace ciphertext accepted")
	}
	stored.ID = "other"
	if err = db.Create(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = v.Get("other"); err == nil {
		t.Fatal("ciphertext moved between credentials")
	}
}

func TestEnterpriseRejectsInvalidConfiguration(t *testing.T) {
	h := newHarness(t)
	for _, raw := range []string{"http://company.example", "https://user:secret@company.example", "https://company.example/path", "https://company.example?x=1"} {
		if _, err := Origin(raw, false); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	c := h.s.Config
	c.AllowedSubjects = nil
	if err := c.Validate(); err == nil {
		t.Fatal("missing membership policy accepted")
	}
	c = h.s.Config
	c.Key = []byte("short")
	if err := c.Validate(); err == nil {
		t.Fatal("invalid encryption key accepted")
	}
}

// Optional real-PostgreSQL test exercises search_path isolation, including raw
// Table("bots") calls and independent default Bot IDs. Use a disposable DB.
func TestPostgresWorkspaces(t *testing.T) {
	dsn := os.Getenv("SOREN_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("SOREN_TEST_POSTGRES_URL not configured")
	}
	db, err := store.OpenDB(&config.Config{StorageDriver: "postgres", DatabaseURL: dsn})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	c := Config{DatabaseURL: dsn, DataDir: t.TempDir(), Key: bytes.Repeat([]byte{1}, 32), Concurrency: 2}
	ws := NewWorkspaces(c, db, llm.NewMockProvider(), "test", "local-echo")
	ws.Active = func(string) error { return nil }
	for i, id := range []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"} {
		schema := "workspace_" + strings.ReplaceAll(id, "-", "")
		defer db.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`)
		handler, err := ws.Handler(User{ID: id})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", "/api/bots", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 || bytes.Contains(rec.Body.Bytes(), []byte("only first")) {
			t.Fatalf("workspace %d: %d %s", i, rec.Code, rec.Body.String())
		}
		if i == 0 {
			req = httptest.NewRequest("POST", "/api/bots", strings.NewReader(`{"name":"only first","avatar":"soren","instructions":"a","status":"active","version":1}`))
			req.Header.Set("Content-Type", "application/json")
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != 201 {
				t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
			}
			// A process restart must migrate existing tables without losing data.
			restarted := NewWorkspaces(c, db, llm.NewMockProvider(), "test", "local-echo")
			restarted.Active = ws.Active
			reopened, err := restarted.Handler(User{ID: id})
			if err != nil {
				t.Fatalf("reopen workspace: %v", err)
			}
			rec = httptest.NewRecorder()
			reopened.ServeHTTP(rec, httptest.NewRequest("GET", "/api/bots", nil))
			if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("only first")) {
				t.Fatal("workspace data lost after restart")
			}
		}
	}
}

func TestModelLimitUsesCompanyModelAndChecksDisabledUser(t *testing.T) {
	allowed := true
	p := &limitedProvider{inner: llm.NewMockProvider(), slots: make(chan struct{}, 1), model: "company-model", active: func() error {
		if !allowed {
			return fmt.Errorf("disabled")
		}
		return nil
	}}
	options := p.options(&llm.Options{Model: "personal-model", MaxTokens: 100000})
	if options.Model != "company-model" || options.MaxTokens != 8192 {
		t.Fatal("client overrode company model limits")
	}
	allowed = false
	if _, err := p.Chat(context.Background(), nil, nil); err == nil {
		t.Fatal("disabled user invoked model")
	}
}

func TestSaveUserReusesIdentityAndKeepsDisabledAccountsDisabled(t *testing.T) {
	h := newHarness(t)
	first, err := h.s.saveUser("alice", "First Name")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.s.saveUser("alice", "Updated Name")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("existing identity changed")
	}
	var stored User
	if err := h.db.First(&stored, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Updated Name" {
		t.Fatal("display name not refreshed")
	}
	if err := h.db.Model(&stored).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.saveUser("alice", "Must Not Apply"); !errors.Is(err, errUnauthorized) {
		t.Fatalf("disabled account accepted: %v", err)
	}
	h.db.First(&stored, "id = ?", first.ID)
	if !stored.Disabled || stored.Name != "Updated Name" {
		t.Fatal("disabled identity was modified")
	}
}

func TestSaveUserConcurrentFirstLogin(t *testing.T) {
	h := newHarness(t)
	const requests = 6
	results := make(chan User, requests)
	failures := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u, err := h.s.saveUser("alice", "Alice")
			if err != nil {
				failures <- err
				return
			}
			results <- u
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Errorf("concurrent login: %v", err)
	}
	var id string
	for u := range results {
		if id != "" && id != u.ID {
			t.Error("concurrent login created another identity")
		}
		id = u.ID
	}
	var count int64
	if err := h.db.Model(&User{}).Where("subject = ?", "alice").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("created %d identities", count)
	}
}
