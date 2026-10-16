package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
	"golang.org/x/oauth2"
)

type scripted struct {
	calls      int
	name, args string
	tools      []llm.Tool
}

func (p *scripted) Chat(_ context.Context, m []llm.Message, opts *llm.Options) (*llm.Response, error) {
	p.calls++
	p.tools = opts.Tools
	if m[len(m)-1].Role == "tool" {
		return &llm.Response{Content: "完成"}, nil
	}
	return &llm.Response{ToolCalls: []llm.ToolCall{{ID: "call-1", Type: "function", Function: llm.Function{Name: p.name, Arguments: p.args}}}}, nil
}
func (*scripted) StreamChat(context.Context, []llm.Message, *llm.Options, chan<- llm.StreamChunk) error {
	return nil
}
func fixture(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	db, e := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(dir, "test.db")})
	if e != nil {
		t.Fatal(e)
	}
	for _, sql := range []string{"CREATE TABLE bots (id TEXT PRIMARY KEY, status TEXT)", "INSERT INTO bots VALUES ('a','active'),('b','active')", "CREATE TABLE runs (id TEXT PRIMARY KEY, owner_bot_id TEXT, status TEXT)", "INSERT INTO runs VALUES ('r','a','running')"} {
		if e = db.Exec(sql).Error; e != nil {
			t.Fatal(e)
		}
	}
	s, e := New(db, &MemoryVault{}, dir)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func gmail(t *testing.T, s *Service) *scripted {
	c := Connection{ID: "test", Kind: "gmail", Name: "Gmail", AllowedBots: []string{"a"}, Enabled: true, Version: 1}
	if e := s.DB.Create(&c).Error; e != nil {
		t.Fatal(e)
	}
	if e := s.Vault.Set(c.ID, Credential{Token: &oauth2.Token{AccessToken: "test", Expiry: time.Now().Add(time.Hour)}}); e != nil {
		t.Fatal(e)
	}
	return &scripted{name: "c_test_send", args: `{"to":"test@example.com","subject":"Test","body":"Original approved body"}`}
}
func loop(s *Service, p *scripted) (*llm.Response, error) {
	return s.Loop(context.Background(), p, "a", "r", 0, []llm.Message{{Role: "user", Content: "task"}}, &llm.Options{Model: "test"})
}
func pending(t *testing.T, s *Service) Invocation {
	var inv Invocation
	if e := s.DB.First(&inv).Error; e != nil {
		t.Fatal(e)
	}
	return inv
}

func TestNewTurnDoesNotReplayConversationHistory(t *testing.T) {
	for _, oldToolCall := range []bool{false, true} {
		t.Run(fmt.Sprintf("oldToolCall=%t", oldToolCall), func(t *testing.T) {
			s := fixture(t)
			previous := llm.Message{Role: "assistant", Content: "看不到任何可调用的工具"}
			if oldToolCall {
				// Even incomplete historical calls belong to a different run.
				previous.ToolCalls = []llm.ToolCall{{ID: "old-call", Type: "function", Function: llm.Function{Name: "workspace_create", Arguments: `{"title":"Old","content":"Do not execute again"}`}}}
			}
			initial := []llm.Message{{Role: "user", Content: "旧问题"}, previous, {Role: "user", Content: "现在调用已连接的工具"}}
			p := &scripted{name: "workspace_create", args: `{"title":"New","content":"Current result"}`}
			for attempt := 0; attempt < 2; attempt++ {
				response, err := s.Loop(context.Background(), p, "a", "r", 0, initial, &llm.Options{Model: "test"})
				if err != nil {
					t.Fatal(err)
				}
				if response.Content != "完成" {
					t.Fatalf("returned historical response: %q", response.Content)
				}
			}
			var artifacts []Artifact
			if err := s.DB.Find(&artifacts).Error; err != nil {
				t.Fatal(err)
			}
			if p.calls != 2 || len(p.tools) == 0 || len(artifacts) != 1 || artifacts[0].Content != "Current result" {
				t.Fatalf("model calls=%d tools=%d artifacts=%+v", p.calls, len(p.tools), artifacts)
			}
		})
	}
}

func TestLocalArtifactIsRealAndReplayDoesNotDuplicate(t *testing.T) {
	s := fixture(t)
	p := &scripted{name: "workspace_create", args: `{"title":"Weekly report","content":"# Actual saved content"}`}
	if _, e := loop(s, p); e != nil {
		t.Fatal(e)
	}
	if _, e := loop(s, p); e != nil {
		t.Fatal(e)
	}
	var n int64
	s.DB.Model(&Artifact{}).Count(&n)
	if n != 1 || p.calls != 2 {
		t.Fatalf("artifacts=%d model calls=%d", n, p.calls)
	}
	var a Artifact
	s.DB.First(&a)
	if a.Content != "# Actual saved content" || a.BotID != "a" {
		t.Fatal(a)
	}
}
func TestApprovalIsBoundAndResumesWithoutDuplicateSend(t *testing.T) {
	s := fixture(t)
	p := gmail(t, s)
	initial := []llm.Message{{Role: "user", Content: "Old request"}, {Role: "assistant", Content: "Old response"}, {Role: "user", Content: "Send the email"}}
	run := func(service *Service) (*llm.Response, error) {
		return service.Loop(context.Background(), p, "a", "r", 0, initial, &llm.Options{Model: "test"})
	}
	sent := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent++
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing credential")
		}
		fmt.Fprint(w, `{"id":"receipt"}`)
	}))
	defer server.Close()
	s.GoogleAPI = server.URL
	if _, e := run(s); !errors.Is(e, ErrApproval) {
		t.Fatal(e)
	}
	if sent != 0 {
		t.Fatal("sent before approval")
	}
	inv := pending(t, s)
	s.DB.Exec("UPDATE runs SET status='waiting_approval'")
	if e := s.Decide(inv.ID, "b", true); e == nil {
		t.Fatal("wrong owner approved")
	}
	if e := s.Decide(inv.ID, "a", true); e != nil {
		t.Fatal(e)
	}
	// A new Service simulates process restart; the assistant call transcript persists.
	next, e := New(s.DB, s.Vault, s.DataDir)
	if e != nil {
		t.Fatal(e)
	}
	next.GoogleAPI = server.URL
	s.DB.Exec("UPDATE runs SET status='running'")
	if _, e = run(next); e != nil {
		t.Fatal(e)
	}
	if _, e = run(next); e != nil {
		t.Fatal(e)
	}
	if sent != 1 || p.calls != 2 {
		t.Fatalf("sent=%d model calls=%d", sent, p.calls)
	}
}
func TestDeniedOrUncertainStopsModelLoop(t *testing.T) {
	for _, denied := range []bool{true, false} {
		t.Run(fmt.Sprint(denied), func(t *testing.T) {
			s := fixture(t)
			p := gmail(t, s)
			sent := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { sent++; w.WriteHeader(503) }))
			defer server.Close()
			s.GoogleAPI = server.URL
			_, _ = loop(s, p)
			inv := pending(t, s)
			s.DB.Exec("UPDATE runs SET status='waiting_approval'")
			if e := s.Decide(inv.ID, "a", !denied); e != nil {
				t.Fatal(e)
			}
			s.DB.Exec("UPDATE runs SET status='running'")
			_, e := loop(s, p)
			if denied && !errors.Is(e, ErrDenied) {
				t.Fatal(e)
			}
			if !denied && !errors.Is(e, ErrUncertain) {
				t.Fatal(e)
			}
			_, _ = loop(s, p)
			if p.calls != 1 {
				t.Fatal("model continued after denial/uncertainty")
			}
			want := 1
			if denied {
				want = 0
			}
			if sent != want {
				t.Fatalf("unexpected retries: %d", sent)
			}
		})
	}
}
func TestRevokedConnectionCannotExecuteApprovedCall(t *testing.T) {
	s := fixture(t)
	p := gmail(t, s)
	_, _ = loop(s, p)
	inv := pending(t, s)
	s.DB.Exec("UPDATE runs SET status='waiting_approval'")
	_ = s.Decide(inv.ID, "a", true)
	s.DB.Model(&Connection{}).Where("id = ?", "test").Update("version", 2)
	s.DB.Exec("UPDATE runs SET status='running'")
	cat, _ := s.catalog("a")
	_, e := s.invoke(context.Background(), cat, "a", "r", inv.ID, llm.ToolCall{Function: llm.Function{Name: p.name, Arguments: p.args}})
	if e == nil {
		t.Fatal("stale approval accepted")
	}
	other, _ := s.catalog("b")
	if _, ok := other[p.name]; !ok {
		t.Fatal("workspace connection missing from other bot")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(text string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(text))}
}
func TestGitHubDeviceFlowIdentityBindingAndRateLimit(t *testing.T) {
	s := fixture(t)
	tokenPolls := 0
	githubID := 42
	s.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/login/device/code":
			return reply(`{"device_code":"private-code","user_code":"ABCD-EFGH","expires_in":900,"interval":5}`), nil
		case "/login/oauth/access_token":
			tokenPolls++
			return reply(`{"access_token":"private-token"}`), nil
		case "/user":
			return reply(fmt.Sprintf(`{"id":%d,"login":"tester","name":"Test"}`, githubID)), nil
		}
		return nil, errors.New("unexpected URL")
	})}
	code, e := s.BeginGitHub(context.Background(), "client-123")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(code)
	if strings.Contains(string(raw), "private-code") {
		t.Fatal("device secret leaked")
	}
	if identity, e := s.PollGitHub(context.Background(), code.ID); e != nil || identity != nil || tokenPolls != 0 {
		t.Fatal("polled too early")
	}
	s.pending[code.ID].Next = time.Time{}
	identity, e := s.PollGitHub(context.Background(), code.ID)
	if e != nil || identity.GitHubID != 42 {
		t.Fatal(identity, e)
	}
	githubID = 99
	code, _ = s.BeginGitHub(context.Background(), "client-123")
	s.pending[code.ID].Next = time.Time{}
	if _, e = s.PollGitHub(context.Background(), code.ID); e == nil {
		t.Fatal("different account merged into local space")
	}
}
func TestGooglePKCEStateSingleUse(t *testing.T) {
	s := fixture(t)
	s.HTTP = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/token" {
			_ = r.ParseForm()
			if r.Form.Get("code_verifier") == "" {
				t.Error("missing verifier")
			}
			return reply(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`), nil
		}
		return reply(`{"emailAddress":"tester@example.com"}`), nil
	})}
	target, e := s.BeginGoogle(Credential{ClientID: "test.apps.googleusercontent.com", ClientSecret: "test-secret"}, "http://127.0.0.1:8080/oauth/google/callback")
	if e != nil {
		t.Fatal(e)
	}
	parsed, _ := url.Parse(target)
	state := parsed.Query().Get("state")
	if parsed.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("missing PKCE")
	}
	if e = s.CompleteGoogle(context.Background(), "wrong", "code"); e == nil {
		t.Fatal("accepted wrong state")
	}
	if e = s.CompleteGoogle(context.Background(), state, "code"); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteGoogle(context.Background(), state, "code"); e == nil {
		t.Fatal("replayed state")
	}
	var c Connection
	s.DB.First(&c)
	if c.Account != "tester@example.com" {
		t.Fatal(c)
	}
	credential, e := s.Vault.Get(c.ID)
	if e != nil || credential.Token.RefreshToken != "refresh" {
		t.Fatal("token not persisted")
	}
}
func TestRemoteMCPRejectsLocalTargets(t *testing.T) {
	s := fixture(t)
	for _, endpoint := range []string{"http://example.com/mcp", "https://user:pass@example.com/mcp", "https://127.0.0.1/mcp"} {
		if _, e := s.mcpSession(context.Background(), endpoint, ""); e == nil {
			t.Fatal("accepted unsafe endpoint", endpoint)
		}
	}
}

func TestToolArgumentsAreValidatedBeforeApprovalOrExecution(t *testing.T) {
	s := fixture(t)
	p := gmail(t, s)
	catalog, e := s.catalog("a")
	if e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"to":null,"subject":"test","body":"text"}`, `{"to":"test@example.com","body":"text"}`, `{"to":"test@example.com","subject":"test","body":"text","draft":false}`} {
		_, e = s.invoke(context.Background(), catalog, "a", "r", "bad", llm.ToolCall{Function: llm.Function{Name: p.name, Arguments: raw}})
		if e == nil || errors.Is(e, ErrApproval) {
			t.Fatal("invalid arguments reached approval")
		}
	}
	var count int64
	s.DB.Model(&Invocation{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid call was persisted as executable")
	}
	var schema map[string]any
	_ = json.Unmarshal(catalog["workspace_list"].Tool.Function.Parameters, &schema)
	if _, ok := schema["required"].([]any); !ok {
		t.Fatal("empty required must serialize as array")
	}
}
