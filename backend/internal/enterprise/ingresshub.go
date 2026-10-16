package enterprise

import (
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Trust anchors are fixed for this deployment mode, never taken from a token,
// Host/X-Forwarded-Host header, browser parameter, or discovery response.
const ingressHubIssuer = "https://sso.app.chaitin.net"

// 公钥走同一网关的内网入口，避免集群访问公网 ALB 超时；JWT 的 issuer 仍为上面的固定值。
const ingressHubJWKS = "https://sso.app.in.chaitin.net/.well-known/jwks.json"

type ingressProfile struct {
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	Subject   string          `json:"sub"`
	Name      string          `json:"name"`
	Username  string          `json:"preferred_username"`
	IssuedAt  int64           `json:"iat"`
	Expires   int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
}

type profileVerifier struct {
	mu                  sync.Mutex
	keys                map[string]ed25519.PublicKey
	expires, retryAfter time.Time
}

// Cache for five minutes. Unknown kids trigger a bounded refresh for rotation;
// outages are retryable, unlike sync.Once, and expired keys fail closed.
func (v *profileVerifier) key(ctx context.Context, client *http.Client, kid string) (ed25519.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	if key := v.keys[kid]; key != nil && now.Before(v.expires) {
		return key, nil
	}
	if now.Before(v.retryAfter) {
		return nil, errors.New("identity keys unavailable")
	}
	v.retryAfter = now.Add(10 * time.Second)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ingressHubJWKS, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("identity keys unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return nil, errors.New("invalid identity keys")
	}
	var doc struct {
		Keys []struct{ Kid, Kty, Crv, Alg, Use, X string }
	}
	if json.Unmarshal(body, &doc) != nil {
		return nil, errors.New("invalid identity keys")
	}
	keys := make(map[string]ed25519.PublicKey)
	for _, k := range doc.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" || k.Alg != "EdDSA" || (k.Use != "" && k.Use != "sig") || len(k.Kid) > 256 {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		if _, duplicate := keys[k.Kid]; duplicate {
			return nil, errors.New("ambiguous identity key")
		}
		keys[k.Kid] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, errors.New("invalid identity keys")
	}
	v.keys = keys
	v.expires = now.Add(5 * time.Minute)
	if key := keys[kid]; key != nil {
		return key, nil
	}
	return nil, errors.New("unknown identity key")
}

func (s *Server) ingressIdentity(r *http.Request) (ingressProfile, error) {
	var p ingressProfile
	invalid := errors.New("invalid gateway identity")
	raw := r.Header.Get("X-IngressHub-Profile")
	if len(raw) == 0 || len(raw) > 16<<10 {
		return p, invalid
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return p, invalid
	}
	var header struct {
		Alg, Kid string
		Crit     []string
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(b, &header) != nil || header.Alg != "EdDSA" || len(header.Kid) > 256 || len(header.Crit) != 0 {
		return p, invalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return p, invalid
	}
	key, err := s.profiles.key(r.Context(), s.HTTP, header.Kid)
	if err != nil {
		return p, err
	}
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return p, invalid
	}
	b, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(b, &p) != nil {
		return p, invalid
	}
	now := time.Now().Unix()
	if p.Issuer != ingressHubIssuer || p.Subject == "" || len(p.Subject) > 256 || p.IssuedAt <= 0 || p.IssuedAt > now+30 || p.Expires <= now || p.Expires <= p.IssuedAt || p.NotBefore > now+30 {
		return p, invalid
	}
	root, _ := url.Parse(s.Config.PublicURL)
	var audience string
	var audiences []string
	if json.Unmarshal(p.Audience, &audience) == nil {
		audiences = []string{audience}
	} else if json.Unmarshal(p.Audience, &audiences) != nil {
		return p, invalid
	}
	matched := false
	for _, a := range audiences {
		if a == root.Host {
			matched = true
		}
	}
	if !matched || (!s.Config.AllowedSubjects["*"] && !s.Config.AllowedSubjects[p.Subject]) {
		return p, invalid
	}
	if p.Name == "" {
		p.Name = p.Username
	}
	if p.Name == "" {
		p.Name = "公司成员"
	}
	if len(p.Name) > 256 {
		return p, invalid
	}
	return p, nil
}

var ingressConfirmation = template.Must(template.New("confirm").Parse(`<!doctype html><html lang="zh"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>登录 Soren</title><body style="font-family:system-ui;max-width:32rem;margin:12vh auto;padding:2rem;line-height:1.7"><h1>登录公司空间</h1><p>公司：{{.Company}}</p><p>当前账号：{{.Name}}</p><p>仅当你刚刚在自己的 Soren 客户端发起登录时，才确认本次连接。请勿确认他人发送的登录链接。</p><form method="post" action="/auth/confirm"><input type="hidden" name="ticket" value="{{.Ticket}}"><input type="hidden" name="state" value="{{.State}}"><button type="submit">确认登录我的 Soren</button></form><p>取消登录请关闭此页面。</p></body></html>`))

func loginCookieName(a *attempt) string { return "soren_login_" + a.ID[:12] }

func (s *Server) browserIngressHub(w http.ResponseWriter, r *http.Request) {
	p, err := s.ingressIdentity(r)
	if err != nil {
		fail(w, 403, "公司身份验证失败，请重新登录或联系管理员")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.attempts[r.URL.Query().Get("ticket")]
	if a == nil || time.Now().After(a.Expires) || a.Started {
		fail(w, 400, "登录链接已失效，请回到 Soren 重新登录")
		return
	}
	cookie := secret()
	a.Started = true
	a.Subject = p.Subject
	a.State = secret()
	a.CookieHash = hash(cookie)
	http.SetCookie(w, &http.Cookie{Name: loginCookieName(a), Value: cookie, Path: "/auth/confirm", HttpOnly: true, Secure: !s.Config.Development, SameSite: http.SameSiteStrictMode, MaxAge: 300})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// no-referrer 会让浏览器的表单 POST 带 Origin: null，与确认接口的
	// 同源校验冲突。仅此页允许同源 Referer，跨站仍不发送登录链接。
	w.Header().Set("Referrer-Policy", "same-origin")
	_ = ingressConfirmation.Execute(w, struct{ Company, Name, Ticket, State string }{s.Config.Name, p.Name, a.ID, a.State})
}

func (s *Server) confirmIngressHub(w http.ResponseWriter, r *http.Request) {
	// Both browser endpoints must remain behind platform SSO. Never accept the
	// gateway JWT on /api/* as a substitute for the desktop's revocable session.
	if r.Header.Get("Origin") != s.Config.PublicURL {
		fail(w, 403, "登录确认来源无效")
		return
	}
	p, err := s.ingressIdentity(r)
	if err != nil {
		fail(w, 403, "公司身份验证失败，请重新登录或联系管理员")
		return
	}
	if r.ParseForm() != nil {
		fail(w, 400, "登录请求无效")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.attempts[r.PostForm.Get("ticket")]
	if a == nil || !a.Started || a.Consumed || time.Now().After(a.Expires) || a.Subject != p.Subject || subtle.ConstantTimeCompare([]byte(a.State), []byte(r.PostForm.Get("state"))) != 1 {
		fail(w, 400, "登录确认无效或已过期")
		return
	}
	cookie, err := r.Cookie(loginCookieName(a))
	if err != nil || subtle.ConstantTimeCompare([]byte(hash(cookie.Value)), []byte(a.CookieHash)) != 1 {
		fail(w, 400, "请使用发起登录的浏览器确认")
		return
	}
	a.Consumed = true
	http.SetCookie(w, &http.Cookie{Name: loginCookieName(a), Path: "/auth/confirm", MaxAge: -1, HttpOnly: true, Secure: !s.Config.Development, SameSite: http.SameSiteStrictMode})
	user, err := s.saveUser(p.Subject, p.Name)
	if err != nil {
		status := http.StatusServiceUnavailable
		a.Error = "公司服务暂不可用，请稍后重新登录或联系管理员"
		if errors.Is(err, errUnauthorized) {
			status = http.StatusForbidden
			a.Error = "账号已停用或未获公司授权，请联系管理员"
		}
		loginPage(w, status, "登录未完成", a.Error)
		return
	}
	token := secret()
	if s.DB.Create(&session{Hash: hash(token), UserID: user.ID, ExpiresAt: time.Now().Add(s.Config.SessionTTL)}).Error != nil {
		a.Error = "登录会话保存失败，请重试"
		loginPage(w, 503, "登录未完成", a.Error)
		return
	}
	a.Token = token
	a.User = user
	_ = s.DB.Where("expires_at < ?", time.Now()).Delete(&session{}).Error
	loginPage(w, 200, "登录成功", "请返回 Soren 核对账号并进入公司空间。可以关闭此页面。")
}
