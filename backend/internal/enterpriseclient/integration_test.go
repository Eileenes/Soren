package enterpriseclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/enterprise"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
)

// Complete desktop -> company -> IdP -> PostgreSQL journey. All identities and
// credentials are synthetic; callers must supply a disposable PostgreSQL DB.
func TestPostgresDesktopOAuthJourney(t *testing.T) {
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
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/authorize":
			q := r.URL.Query()
			http.Redirect(w, r, q.Get("redirect_uri")+"?code=journey-user&state="+url.QueryEscape(q.Get("state")), 302)
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"journey-user","token_type":"Bearer"}`))
		case "/userinfo":
			_, _ = w.Write([]byte(`{"sub":"journey-user","name":"Journey Employee"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer idp.Close()
	upstream := httptest.NewUnstartedServer(nil)
	root := "http://" + upstream.Listener.Addr().String()
	cfg := enterprise.Config{PublicURL: root, Name: "Journey Company", DatabaseURL: dsn, DataDir: t.TempDir(), AuthorizationURL: idp.URL + "/authorize", TokenURL: idp.URL + "/token", UserInfoURL: idp.URL + "/userinfo", ClientID: "test-client", ClientSecret: "test-secret", SubjectClaim: "sub", Scope: "openid profile", AuthMethod: "client_secret_basic", AllowedSubjects: map[string]bool{"journey-user": true}, Key: bytes.Repeat([]byte{9}, 32), Development: true, SessionTTL: time.Hour, Concurrency: 2}
	spaces := enterprise.NewWorkspaces(cfg, db, llm.NewMockProvider(), "test", "local-echo")
	server, err := enterprise.New(cfg, db, spaces.Handler)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enterprise.New(cfg, db, spaces.Handler); err != nil {
		t.Fatalf("reinitialize existing membership tables: %v", err)
	}
	spaces.Active = server.Active
	upstream.Config.Handler = server.Handler()
	upstream.Start()
	defer upstream.Close()
	local, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "desktop.db")})
	if err != nil {
		t.Fatal(err)
	}
	localSQL, _ := local.DB()
	defer localSQL.Close()
	bridge, err := New(local, &connectors.MemoryVault{}, true)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	bridge.Register(router)
	var scope string
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Soren-Workspace", scope)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var start struct{ ID, URL string }
	_ = json.Unmarshal(call("POST", "/api/enterprise/login", map[string]string{"serverUrl": root}, 200), &start)
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	res, err := browser.Get(start.URL)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(page, []byte("登录成功")) {
		t.Fatalf("browser callback: %d %s", res.StatusCode, page)
	}
	call("POST", "/api/enterprise/poll", map[string]string{"id": start.ID}, 200)
	if raw := call("GET", "/api/enterprise/status", nil, 200); !bytes.Contains(raw, []byte(`"mode":"personal"`)) {
		t.Fatal("entered company before confirmation")
	}
	var activated struct {
		Connection Record `json:"connection"`
	}
	_ = json.Unmarshal(call("POST", "/api/enterprise/complete", map[string]string{"id": start.ID}, 200), &activated)
	u := activated.Connection
	defer db.Exec(`DROP SCHEMA IF EXISTS "workspace_` + strings.ReplaceAll(u.WorkspaceID, "-", "") + `" CASCADE`)
	defer db.Exec("DELETE FROM enterprise_sessions WHERE user_id = ?", u.UserID)
	defer db.Delete(&enterprise.User{}, "id = ?", u.UserID)
	scope = root + ":" + u.WorkspaceID + ":" + u.UserID
	if raw := call("GET", "/enterprise/api/bots", nil, 200); !bytes.Contains(raw, []byte(`"id":"soren"`)) {
		t.Fatal("default bot missing")
	}
	call("POST", "/enterprise/api/chat", map[string]string{"message": "private PostgreSQL journey"}, 200)
	if raw := call("GET", "/enterprise/api/conversations", nil, 200); !bytes.Contains(raw, []byte("private PostgreSQL journey")) {
		t.Fatal("chat not persisted")
	}
	call("GET", "/enterprise/api/connections", nil, 200)
	call("POST", "/api/enterprise/disconnect", map[string]any{}, 200)
	call("GET", "/enterprise/api/bots", nil, 401)
}
