package enterprise

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type attempt struct {
	ID, PollHash, State, CookieHash, Verifier string
	Expires                                   time.Time
	Started, Consumed                         bool
	Token, Error                              string
	User                                      User
	Subject                                   string
}

func (s *Server) oauth() *oauth2.Config {
	style := oauth2.AuthStyleInHeader
	if s.Config.AuthMethod == "client_secret_post" {
		style = oauth2.AuthStyleInParams
	}
	return &oauth2.Config{ClientID: s.Config.ClientID, ClientSecret: s.Config.ClientSecret, RedirectURL: s.Config.PublicURL + "/auth/callback", Scopes: strings.Fields(s.Config.Scope), Endpoint: oauth2.Endpoint{AuthURL: s.Config.AuthorizationURL, TokenURL: s.Config.TokenURL, AuthStyle: style}}
}
func (s *Server) startLogin(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimit.Allow() {
		fail(w, 429, "登录请求过多，请稍后重试")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, a := range s.attempts {
		if time.Now().After(a.Expires) {
			delete(s.attempts, id)
		}
	}
	if len(s.attempts) >= 512 {
		fail(w, 429, "登录请求过多，请稍后重试")
		return
	}
	poll := secret()
	a := &attempt{ID: secret(), PollHash: hash(poll), Expires: time.Now().Add(5 * time.Minute)}
	s.attempts[a.ID] = a
	respond(w, 201, map[string]any{"id": a.ID, "pollSecret": poll, "url": s.Config.PublicURL + "/auth/browser?ticket=" + a.ID, "expiresAt": a.Expires})
}
func (s *Server) browserLogin(w http.ResponseWriter, r *http.Request) {
	if s.Config.AuthMode == "ingresshub" {
		s.browserIngressHub(w, r)
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
	a.State = secret()
	a.CookieHash = hash(cookie)
	a.Verifier = oauth2.GenerateVerifier()
	http.SetCookie(w, &http.Cookie{Name: "soren_login_" + a.ID[:12], Value: cookie, Path: "/auth/callback", HttpOnly: true, Secure: !s.Config.Development, SameSite: http.SameSiteLaxMode, MaxAge: 300})
	http.Redirect(w, r, s.oauth().AuthCodeURL(a.State, oauth2.S256ChallengeOption(a.Verifier)), http.StatusFound)
}
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	s.mu.Lock()
	var a *attempt
	for _, candidate := range s.attempts {
		if len(state) == 43 && candidate.State == state {
			a = candidate
			break
		}
	}
	if a == nil || !a.Started || a.Consumed || time.Now().After(a.Expires) {
		s.mu.Unlock()
		fail(w, 400, "登录回调无效或已过期")
		return
	}
	cookie, err := r.Cookie("soren_login_" + a.ID[:12])
	if err != nil || subtle.ConstantTimeCompare([]byte(hash(cookie.Value)), []byte(a.CookieHash)) != 1 {
		s.mu.Unlock()
		fail(w, 400, "登录校验失败，请使用发起登录的浏览器重试")
		return
	}
	a.Consumed = true
	verifier := a.Verifier
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "soren_login_" + a.ID[:12], Path: "/auth/callback", MaxAge: -1, HttpOnly: true, Secure: !s.Config.Development, SameSite: http.SameSiteLaxMode})
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, s.HTTP)
	var user User
	code := r.URL.Query().Get("code")
	if r.URL.Query().Get("error") != "" || code == "" {
		err = fmt.Errorf("授权未完成")
	} else {
		var token *oauth2.Token
		token, err = s.oauth().Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err == nil {
			user, err = s.userInfo(ctx, token.AccessToken)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil || time.Now().After(a.Expires) {
		a.Error = "登录未完成，请确认账号已获公司授权后重试"
		loginPage(w, 400, "登录未完成", "请返回 Soren 重新登录，或联系管理员检查账号授权。")
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
	loginPage(w, 200, "登录成功", "请返回 Soren，进入你的公司空间。可以关闭此页面。")
}
func (s *Server) userInfo(ctx context.Context, token string) (User, error) {
	var zero User
	req, err := http.NewRequestWithContext(ctx, "GET", s.Config.UserInfoURL, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := s.HTTP.Do(req)
	if err != nil {
		return zero, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return zero, fmt.Errorf("用户信息校验失败")
	}
	var claims map[string]any
	d := json.NewDecoder(io.LimitReader(res.Body, 64<<10))
	d.UseNumber()
	if err = d.Decode(&claims); err != nil {
		return zero, err
	}
	claim := func(key string) string {
		switch v := claims[key].(type) {
		case string:
			return v
		case json.Number:
			return string(v)
		}
		return ""
	}
	subject, name := claim(s.Config.SubjectClaim), claim("name")
	if subject == "" || len(subject) > 256 || (!s.Config.AllowedSubjects["*"] && !s.Config.AllowedSubjects[subject]) {
		return zero, fmt.Errorf("账号不在允许范围内")
	}
	if name == "" {
		name = claim("username")
	}
	if name == "" {
		name = "公司成员"
	}
	if len(name) > 256 {
		return zero, fmt.Errorf("姓名字段无效")
	}
	return s.saveUser(subject, name)
}
func (s *Server) pollLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID     string `json:"id"`
		Secret string `json:"pollSecret"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		fail(w, 400, "登录请求无效")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.attempts[in.ID]
	if a == nil || time.Now().After(a.Expires) || subtle.ConstantTimeCompare([]byte(hash(in.Secret)), []byte(a.PollHash)) != 1 {
		fail(w, 401, "登录请求已失效，请重新登录")
		return
	}
	if a.Error != "" {
		fail(w, 403, a.Error)
		return
	}
	if a.Token == "" {
		respond(w, 202, map[string]string{"status": "pending"})
		return
	}
	if err := s.Active(a.User.ID); err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, errUnauthorized) {
			status = http.StatusForbidden
		}
		fail(w, status, err.Error())
		return
	}
	// Repeated polling may return the same session until the short attempt expires.
	// No bearer token is ever sent through the browser redirect or URL.
	out := s.identity(a.User)
	out["token"] = a.Token
	out["status"] = "authenticated"
	respond(w, 200, out)
}
func loginPage(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	// Arguments are fixed application strings, never OAuth responses or user claims.
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=zh><meta charset=utf-8><meta name=viewport content='width=device-width,initial-scale=1'><title>Soren</title><body style='font-family:system-ui;max-width:32rem;margin:15vh auto;padding:2rem'><h1>%s</h1><p>%s</p></body></html>", title, message)
}
