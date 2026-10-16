package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type oauthAttempt struct {
	Kind                     string
	Credential               Credential
	Code, Verifier, Redirect string
	Expires, Next            time.Time
	Interval                 time.Duration
}
type DeviceLogin struct {
	ID       string `json:"id"`
	UserCode string `json:"userCode"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
}

func (s *Service) form(ctx context.Context, endpoint string, values url.Values, out any) error {
	req, e := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(values.Encode()))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, e := s.HTTP.Do(req)
	if e != nil {
		return errors.New("无法连接授权服务")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("授权服务暂不可用")
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
}
func (s *Service) prune() {
	for id, p := range s.pending {
		if time.Now().After(p.Expires) {
			delete(s.pending, id)
		}
	}
}
func (s *Service) BeginGitHub(ctx context.Context, clientID string) (*DeviceLogin, error) {
	clientID = strings.TrimSpace(clientID)
	if len(clientID) < 5 || len(clientID) > 150 {
		return nil, errors.New("请填写 GitHub OAuth App 的 Client ID，并启用 Device Flow")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if len(s.pending) >= 8 {
		return nil, errors.New("请先完成或等待现有授权过期")
	}
	var code struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		Expires    int    `json:"expires_in"`
		Interval   int    `json:"interval"`
	}
	if e := s.form(ctx, "https://github.com/login/device/code", url.Values{"client_id": {clientID}, "scope": {"read:user"}}, &code); e != nil {
		return nil, e
	}
	if code.DeviceCode == "" || code.UserCode == "" || code.Expires <= 0 {
		return nil, errors.New("GitHub 未返回设备码，请检查 Client ID 和 Device Flow 设置")
	}
	if code.Interval < 5 {
		code.Interval = 5
	}
	if code.Expires > 900 {
		code.Expires = 900
	}
	id := uuid.NewString()
	s.pending[id] = &oauthAttempt{Kind: "github", Credential: Credential{ClientID: clientID}, Code: code.DeviceCode, Expires: time.Now().Add(time.Duration(code.Expires) * time.Second), Interval: time.Duration(code.Interval) * time.Second, Next: time.Now().Add(time.Duration(code.Interval) * time.Second)}
	return &DeviceLogin{id, code.UserCode, "https://github.com/login/device", code.Interval}, nil
}
func (s *Service) PollGitHub(ctx context.Context, id string) (*Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	p, ok := s.pending[id]
	if !ok || p.Kind != "github" {
		return nil, errors.New("授权已过期，请重新登录")
	}
	if time.Now().Before(p.Next) {
		return nil, nil
	}
	p.Next = time.Now().Add(p.Interval)
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if e := s.form(ctx, "https://github.com/login/oauth/access_token", url.Values{"client_id": {p.Credential.ClientID}, "device_code": {p.Code}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &token); e != nil {
		return nil, e
	}
	if token.Error == "authorization_pending" {
		return nil, nil
	}
	if token.Error == "slow_down" {
		p.Interval += 5 * time.Second
		p.Next = time.Now().Add(p.Interval)
		return nil, nil
	}
	if token.AccessToken == "" {
		delete(s.pending, id)
		return nil, errors.New("GitHub 授权未完成或已拒绝")
	}
	delete(s.pending, id)
	raw, e := s.request(ctx, "GET", "https://api.github.com/user", token.AccessToken, nil, map[string]string{"Accept": "application/vnd.github+json"})
	if e != nil {
		return nil, errors.New("无法验证 GitHub 身份")
	}
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if json.Unmarshal([]byte(raw), &user) != nil || user.ID <= 0 {
		return nil, errors.New("GitHub 身份无效")
	}
	var old Identity
	if e = s.DB.First(&old, 1).Error; e == nil && old.GitHubID != user.ID {
		return nil, errors.New("此本地空间已绑定其他 GitHub 账号，不能自动合并数据")
	}
	identity := Identity{ID: 1, GitHubID: user.ID, Login: user.Login, Name: user.Name, BoundAt: time.Now()}
	if e = s.DB.Save(&identity).Error; e != nil {
		return nil, e
	}
	// The access token is only used for identity verification, never persisted or
	// returned to the browser. This binding is not a remote device session.
	return &identity, nil
}
func googleConfig(c Credential, redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: redirect, Scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}}
}
func (s *Service) BeginGoogle(c Credential, redirect string) (string, error) {
	if !strings.HasSuffix(c.ClientID, ".apps.googleusercontent.com") || len(c.ClientID) > 250 || c.ClientSecret == "" {
		return "", errors.New("请填写 Google 桌面 OAuth 客户端的 Client ID 和 Client Secret")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if len(s.pending) >= 8 {
		return "", errors.New("待完成授权过多")
	}
	state := uuid.NewString()
	verifier := oauth2.GenerateVerifier()
	s.pending[state] = &oauthAttempt{Kind: "google", Credential: c, Verifier: verifier, Redirect: redirect, Expires: time.Now().Add(10 * time.Minute)}
	return googleConfig(c, redirect).AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "consent")), nil
}
func (s *Service) CompleteGoogle(ctx context.Context, state, code string) error {
	s.mu.Lock()
	s.prune()
	p, ok := s.pending[state]
	if ok {
		delete(s.pending, state)
	}
	s.mu.Unlock()
	if !ok || p.Kind != "google" || code == "" {
		return errors.New("授权请求已过期或不匹配")
	}
	token, e := googleConfig(p.Credential, p.Redirect).Exchange(context.WithValue(ctx, oauth2.HTTPClient, s.HTTP), code, oauth2.VerifierOption(p.Verifier))
	if e != nil {
		return errors.New("Google 授权交换失败，请重试")
	}
	raw, e := s.request(ctx, "GET", s.GoogleAPI+"/users/me/profile", token.AccessToken, nil, nil)
	if e != nil {
		return errors.New("无法验证 Gmail 账号，请检查授权权限")
	}
	var profile struct {
		Email string `json:"emailAddress"`
	}
	if json.Unmarshal([]byte(raw), &profile) != nil || profile.Email == "" {
		return errors.New("Gmail 账号无效")
	}
	c := Connection{ID: uuid.NewString(), Name: "Gmail", Kind: "gmail", Account: profile.Email, Enabled: true, Version: 1, AllowedBots: []string{}, Tools: []RemoteTool{}}
	p.Credential.Token = token
	if e = s.Vault.Set(c.ID, p.Credential); e != nil {
		return errors.New("系统凭据库保存失败，连接未建立")
	}
	if e = s.DB.Create(&c).Error; e != nil {
		_ = s.Vault.Delete(c.ID)
		return e
	}
	return nil
}
