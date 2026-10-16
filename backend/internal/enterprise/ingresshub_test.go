package enterprise

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type gatewayFixture struct {
	key         ed25519.PrivateKey
	kid         string
	calls       atomic.Int32
	unavailable atomic.Bool
}

func setupGateway(t *testing.T, s *Server) *gatewayFixture {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	g := &gatewayFixture{key: key, kid: "test-key"}
	s.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// 固定为可从集群访问的内网端点，不随待测常量一起改变，防止回退到公网。
		if r.URL.String() != "https://sso.app.in.chaitin.net/.well-known/jwks.json" {
			t.Errorf("untrusted JWKS URL: %s", r.URL)
			return nil, errUnauthorized
		}
		g.calls.Add(1)
		status := 200
		body := `{"keys":[{"kid":"` + g.kid + `","kty":"OKP","crv":"Ed25519","alg":"EdDSA","use":"sig","x":"` + base64.RawURLEncoding.EncodeToString(g.key.Public().(ed25519.PublicKey)) + `"}]}`
		if g.unavailable.Load() {
			status = 503
			body = "unavailable"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	return g
}
func (g *gatewayFixture) token(root, subject string, changes map[string]any) string {
	u, _ := url.Parse(root)
	claims := map[string]any{"iss": ingressHubIssuer, "aud": u.Host, "sub": subject, "name": subject + " Employee", "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix()}
	for k, v := range changes {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": g.kid})
	payload, _ := json.Marshal(claims)
	msg := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return msg + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(g.key, []byte(msg)))
}

func TestIngressHubProfileValidation(t *testing.T) {
	h := newHarness(t, "ingresshub")
	g := setupGateway(t, h.s)
	for _, tc := range []struct {
		name    string
		changes map[string]any
		valid   bool
	}{
		{"valid", nil, true}, {"wrong issuer", map[string]any{"iss": "https://attacker.example"}, false},
		{"JWKS host is not issuer", map[string]any{"iss": "https://sso.app.in.chaitin.net"}, false},
		{"wrong audience", map[string]any{"aud": "other.app.in.chaitin.net"}, false},
		{"expired", map[string]any{"exp": time.Now().Add(-time.Second).Unix()}, false},
		{"missing expiry", map[string]any{"exp": nil}, false}, {"missing issued", map[string]any{"iat": nil}, false},
		{"missing subject", map[string]any{"sub": nil}, false}, {"disallowed user", map[string]any{"sub": "mallory"}, false},
		{"future issued", map[string]any{"iat": time.Now().Add(time.Hour).Unix()}, false},
		{"future nbf", map[string]any{"nbf": time.Now().Add(time.Hour).Unix()}, false},
		{"invalid lifetime", map[string]any{"iat": time.Now().Add(-time.Hour).Unix(), "exp": time.Now().Add(-2 * time.Hour).Unix()}, false},
		{"string expiry", map[string]any{"exp": "9999999999"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", h.server.URL+"/auth/browser", nil)
			r.Header.Set("X-IngressHub-Profile", g.token(h.server.URL, "alice", tc.changes))
			_, err := h.s.ingressIdentity(r)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
	valid := g.token(h.server.URL, "alice", nil)
	parts := strings.Split(valid, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"bob"}`))
	for _, raw := range []string{"", "not-a-jwt", strings.Join(parts, "."), "eyJhbGciOiJub25lIn0.e30.", strings.Repeat("a", 17000)} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-IngressHub-Profile", raw)
		if _, err := h.s.ingressIdentity(r); err == nil {
			t.Fatal("accepted forged identity")
		}
	}
	if g.calls.Load() != 1 {
		t.Fatalf("JWKS fetched %d times", g.calls.Load())
	}
}

func TestIngressHubKeyOutageAndRotation(t *testing.T) {
	h := newHarness(t, "ingresshub")
	g := setupGateway(t, h.s)
	check := func(want bool) {
		t.Helper()
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("X-IngressHub-Profile", g.token(h.server.URL, "alice", nil))
		_, err := h.s.ingressIdentity(r)
		if (err == nil) != want {
			t.Fatalf("want %v got %v", want, err)
		}
	}
	g.unavailable.Store(true)
	check(false)
	check(false)
	if g.calls.Load() != 1 {
		t.Fatal("outage requests were not throttled")
	}
	g.unavailable.Store(false)
	h.s.profiles.retryAfter = time.Time{}
	check(true)
	_, g.key, _ = ed25519.GenerateKey(rand.Reader)
	g.kid = "rotated-key"
	h.s.profiles.retryAfter = time.Time{}
	check(true)
	h.s.profiles.expires = time.Time{}
	h.s.profiles.retryAfter = time.Time{}
	g.unavailable.Store(true)
	check(false)
	g.unavailable.Store(false)
	h.s.profiles.retryAfter = time.Time{}
	check(true)
}

func TestIngressHubBrowserAndDesktopJourney(t *testing.T) {
	h := newHarness(t, "ingresshub")
	g := setupGateway(t, h.s)
	type login struct{ ID, PollSecret, URL string }
	start := func() login {
		t.Helper()
		status, raw := h.request(t, "POST", "/auth/attempts", "", map[string]any{}, nil)
		if status != 201 {
			t.Fatalf("start %d %s", status, raw)
		}
		var a login
		_ = json.Unmarshal(raw, &a)
		return a
	}
	browser := func(a login, subject string) (*http.Cookie, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", a.URL, nil)
		req.Header.Set("X-IngressHub-Profile", g.token(h.server.URL, subject, nil))
		res, err := h.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("browser %d %s", res.StatusCode, b)
		}
		// 浏览器表单 POST 必须保留同源 Origin；no-referrer 会将它变成 null。
		if policy := res.Header.Get("Referrer-Policy"); policy != "same-origin" {
			t.Fatalf("confirmation page referrer policy: %q", policy)
		}
		m := regexp.MustCompile(`name="state" value="([^"]+)"`).FindSubmatch(b)
		if len(m) != 2 || len(res.Cookies()) != 1 {
			t.Fatalf("missing confirmation: %s", b)
		}
		return res.Cookies()[0], string(m[1])
	}
	confirm := func(a login, cookie *http.Cookie, state, subject, origin string, want int) {
		t.Helper()
		form := url.Values{"ticket": {a.ID}, "state": {state}}
		req, _ := http.NewRequest("POST", h.server.URL+"/auth/confirm", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", origin)
		req.Header.Set("X-IngressHub-Profile", g.token(h.server.URL, subject, nil))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := h.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != want {
			t.Fatalf("confirm %d want %d %s", res.StatusCode, want, b)
		}
	}
	poll := func(a login, secret string, want int) map[string]any {
		t.Helper()
		status, raw := h.request(t, "POST", "/auth/poll", "", map[string]string{"id": a.ID, "pollSecret": secret}, nil)
		if status != want {
			t.Fatalf("poll %d %s", status, raw)
		}
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return out
	}
	a := start()
	if status, _ := h.request(t, "GET", "/auth/browser?ticket="+a.ID, "", nil, nil); status != 403 {
		t.Fatal("browser accepted missing gateway identity")
	}
	cookie, state := browser(a, "alice")
	poll(a, a.PollSecret, 202) // GET must not issue a session.
	confirm(a, cookie, state, "alice", "https://attacker.example", 403)
	confirm(a, cookie, state, "alice", "null", 403)
	confirm(a, cookie, state, "alice", "", 403)
	confirm(a, nil, state, "alice", h.server.URL, 400)
	confirm(a, cookie, "wrong-state", "alice", h.server.URL, 400)
	confirm(a, cookie, state, "bob", h.server.URL, 400)
	confirm(a, cookie, state, "alice", h.server.URL, 200)
	confirm(a, cookie, state, "alice", h.server.URL, 400)
	poll(a, "wrong-secret", 401)
	out := poll(a, a.PollSecret, 200)
	token := out["token"].(string)
	if status, _ := h.request(t, "GET", "/api/session", "", nil, map[string]string{"X-IngressHub-Profile": g.token(h.server.URL, "alice", nil)}); status != 401 {
		t.Fatal("gateway header replaced desktop bearer")
	}
	if status, _ := h.request(t, "GET", "/api/bots", token, nil, nil); status != 200 {
		t.Fatalf("bots %d", status)
	}
	if status, _ := h.request(t, "POST", "/api/chat", token, map[string]string{"message": "Alice private message"}, nil); status != 200 {
		t.Fatalf("chat %d", status)
	}
	b := start()
	bc, bs := browser(b, "bob")
	confirm(b, bc, bs, "bob", h.server.URL, 200)
	bob := poll(b, b.PollSecret, 200)["token"].(string)
	if status, raw := h.request(t, "GET", "/api/conversations", bob, nil, nil); status != 200 || strings.Contains(string(raw), "Alice private message") {
		t.Fatalf("cross-user leak %d %s", status, raw)
	}
	if status, _ := h.request(t, "POST", "/api/session/logout", token, nil, nil); status != 204 {
		t.Fatal("logout failed")
	}
	if status, _ := h.request(t, "GET", "/api/session", token, nil, nil); status != 401 {
		t.Fatal("logout did not revoke")
	}
	// 同一 SSO 账号退出后再次登录，应复用原用户和原工作区。
	again := start()
	ac, as := browser(again, "alice")
	confirm(again, ac, as, "alice", h.server.URL, 200)
	againToken := poll(again, again.PollSecret, 200)["token"].(string)
	if status, raw := h.request(t, "GET", "/api/conversations", againToken, nil, nil); status != 200 || !strings.Contains(string(raw), "Alice private message") {
		t.Fatalf("relogin lost existing workspace: %d %s", status, raw)
	}
	var identities int64
	h.db.Model(&User{}).Where("subject = ?", "alice").Count(&identities)
	if identities != 1 {
		t.Fatalf("relogin created %d identities", identities)
	}
	h.s.DB.Model(&User{}).Where("subject = ?", "bob").Update("disabled", true)
	if status, _ := h.request(t, "GET", "/api/session", bob, nil, nil); status != 401 {
		t.Fatal("disabled user retained access")
	}
	if status, _ := h.request(t, "GET", "/auth/callback", "", nil, nil); status != 404 {
		t.Fatal("OAuth callback available in gateway mode")
	}
	expired := start()
	ec, es := browser(expired, "alice")
	h.s.mu.Lock()
	h.s.attempts[expired.ID].Expires = time.Now().Add(-time.Second)
	h.s.mu.Unlock()
	confirm(expired, ec, es, "alice", h.server.URL, 400)
	poll(expired, expired.PollSecret, 401)
}

func TestIngressHubConfigDoesNotRequireOAuthSecrets(t *testing.T) {
	h := newHarness(t)
	c := h.s.Config
	c.AuthMode = "ingresshub"
	c.ClientID = ""
	c.ClientSecret = ""
	c.AuthorizationURL = ""
	c.TokenURL = ""
	c.UserInfoURL = ""
	c.SubjectClaim = ""
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.AuthMode = "invalid"
	if c.Validate() == nil {
		t.Fatal("unknown auth mode accepted")
	}
	c.AuthMode = "oauth"
	if c.Validate() == nil {
		t.Fatal("OAuth accepted missing credentials")
	}
	c.AuthMode = "ingresshub"
	c.AllowedSubjects = nil
	if c.Validate() == nil {
		t.Fatal("missing membership policy accepted")
	}
}
