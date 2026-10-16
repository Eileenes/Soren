package enterpriseclient

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
func TestPostgresDesktopOAuthJourney(t *testing.T)      { testPostgresDesktopJourney(t, "oauth") }
func TestPostgresDesktopIngressHubJourney(t *testing.T) { testPostgresDesktopJourney(t, "ingresshub") }

type journeyTransport func(*http.Request) (*http.Response, error)

func (f journeyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testPostgresDesktopJourney(t *testing.T, mode string) {
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
	pub, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kid": "journey", "kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(pub)}}})
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
	cfg.AuthMode = mode
	if mode == "ingresshub" {
		cfg.ClientID = ""
		cfg.ClientSecret = ""
		cfg.AuthorizationURL = ""
		cfg.TokenURL = ""
		cfg.UserInfoURL = ""
	}
	spaces := enterprise.NewWorkspaces(cfg, db, llm.NewMockProvider(), "test", "local-echo")
	server, err := enterprise.New(cfg, db, spaces.Handler)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enterprise.New(cfg, db, spaces.Handler); err != nil {
		t.Fatalf("reinitialize existing membership tables: %v", err)
	}
	spaces.Active = server.Active
	if mode == "ingresshub" {
		server.HTTP = &http.Client{Transport: journeyTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://sso.app.in.chaitin.net/.well-known/jwks.json" {
				return nil, fmt.Errorf("unexpected identity request")
			}
			req, _ := http.NewRequestWithContext(r.Context(), "GET", idp.URL+"/jwks", nil)
			return http.DefaultTransport.RoundTrip(req)
		})}
	}
	app := server.Handler()
	upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-IngressHub-Profile")
		if mode == "ingresshub" {
			// Simulated edge SSO: native endpoints are exempt; browser endpoints
			// require a cookie and get a signed profile. Never forward that cookie.
			if r.URL.Path == "/test-sso" {
				http.SetCookie(w, &http.Cookie{Name: "test_gateway", Value: "journey-user", Path: "/"})
				http.Redirect(w, r, r.URL.Query().Get("return"), 302)
				return
			}
			if r.URL.Path == "/auth/browser" || r.URL.Path == "/auth/confirm" {
				if _, err := r.Cookie("test_gateway"); err != nil {
					http.Redirect(w, r, "/test-sso?return="+url.QueryEscape(r.URL.RequestURI()), 302)
					return
				}
				host, _ := url.Parse(root)
				header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": "journey"})
				payload, _ := json.Marshal(map[string]any{"iss": "https://sso.app.chaitin.net", "aud": host.Host, "sub": "journey-user", "name": "Journey Employee", "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix()})
				msg := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
				r.Header.Set("X-IngressHub-Profile", msg+"."+base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, []byte(msg))))
				cookies := r.Cookies()
				r.Header.Del("Cookie")
				for _, c := range cookies {
					if c.Name != "test_gateway" {
						r.AddCookie(c)
					}
				}
			}
		}
		app.ServeHTTP(w, r)
	})
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
	if mode == "ingresshub" {
		match := regexp.MustCompile(`name="state" value="([^"]+)"`).FindSubmatch(page)
		if res.StatusCode != 200 || len(match) != 2 {
			t.Fatalf("missing gateway confirmation: %d %s", res.StatusCode, page)
		}
		// Native poll works without the gateway cookie, but cannot activate yet.
		call("POST", "/api/enterprise/poll", map[string]string{"id": start.ID}, 202)
		ticket, _ := url.Parse(start.URL)
		form := url.Values{"ticket": {ticket.Query().Get("ticket")}, "state": {string(match[1])}}
		req, _ := http.NewRequest("POST", root+"/auth/confirm", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", root)
		res, err = browser.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		page, _ = io.ReadAll(res.Body)
		res.Body.Close()
	}
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
