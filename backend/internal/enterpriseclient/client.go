// Package enterpriseclient keeps company credentials in the desktop vault and
// proxies only the selected company's API. Renderer code never receives tokens.
package enterpriseclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/enterprise"
	"gorm.io/gorm"
)

const vaultID = "soren-enterprise-session-v1"

var errSessionInvalid = errors.New("公司登录已失效，请重新登录")

type Record struct {
	ID           uint   `gorm:"primaryKey" json:"-"`
	Active       bool   `json:"active"`
	ServerURL    string `json:"serverUrl"`
	Organization string `json:"organization"`
	WorkspaceID  string `json:"workspaceId"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
}

func (Record) TableName() string { return "enterprise_connection" }
func (r Record) binding() string { return r.ServerURL + ":" + r.WorkspaceID + ":" + r.UserID }

type Discovery struct {
	Product         string `json:"product"`
	ProtocolVersion int    `json:"protocolVersion"`
	Name            string `json:"name"`
	ServerURL       string `json:"serverUrl"`
	Auth            string `json:"auth"`
}
type loginAttempt struct {
	ID         string    `json:"id"`
	Secret     string    `json:"pollSecret"`
	URL        string    `json:"url"`
	Expires    time.Time `json:"expiresAt"`
	ServerURL  string    `json:"-"`
	Token      string    `json:"-"`
	Connection Record    `json:"-"`
}
type identity struct {
	WorkspaceID  string `json:"workspaceId"`
	Organization string `json:"organization"`
	User         struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
}
type Client struct {
	db          *gorm.DB
	vault       connectors.Vault
	development bool
	http        *http.Client
	mu          sync.Mutex
	pending     *loginAttempt
}

func New(db *gorm.DB, v connectors.Vault, development bool) (*Client, error) {
	if err := db.AutoMigrate(&Record{}); err != nil {
		return nil, err
	}
	return &Client{db: db, vault: v, development: development, http: &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (s *Client) request(ctx context.Context, method, target, token string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := s.http.Do(req)
	if err != nil {
		return 0, nil, errors.New("无法连接公司服务器，请检查网络或服务器地址")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if err != nil || len(raw) > 4<<20 || (len(raw) > 0 && !json.Valid(raw)) {
		return 0, nil, errors.New("服务器响应无效或过大")
	}
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		return 0, nil, errors.New("服务器发生重定向，请填写最终的服务根地址")
	}
	return res.StatusCode, raw, nil
}
func (s *Client) record() (Record, error) {
	var r Record
	err := s.db.First(&r, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, nil
	}
	return r, err
}

// Only a definite rejection or identity mismatch invalidates the saved session.
// Network, database and protocol failures leave it available for a later retry.
func (s *Client) checkSession(ctx context.Context, row Record, token string) error {
	status, body, err := s.request(ctx, "GET", row.ServerURL+"/api/session", token, nil)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		return errSessionInvalid
	}
	var user identity
	if status != http.StatusOK || json.Unmarshal(body, &user) != nil || user.User.ID == "" || user.WorkspaceID == "" {
		return errors.New("公司服务暂不可用或响应无效，请稍后重试")
	}
	if user.User.ID != row.UserID || user.WorkspaceID != row.WorkspaceID {
		return errSessionInvalid
	}
	return nil
}
func (s *Client) discover(ctx context.Context, raw string) (Discovery, error) {
	var d Discovery
	root, err := enterprise.Origin(raw, s.development)
	if err != nil {
		return d, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, body, err := s.request(ctx, "GET", root+"/.well-known/soren", "", nil)
	if err != nil {
		return d, err
	}
	if status != 200 || json.Unmarshal(body, &d) != nil || d.Product != "soren" || d.ProtocolVersion != 1 || d.Auth != "oauth-browser" || d.ServerURL != root || strings.TrimSpace(d.Name) == "" || len(d.Name) > 200 {
		return d, errors.New("该地址不是兼容的 Soren 企业服务，请联系管理员确认")
	}
	return d, nil
}
func errorJSON(c *gin.Context, status int, err error) { c.JSON(status, gin.H{"error": err.Error()}) }
func (s *Client) Register(r *gin.Engine) {
	r.GET("/api/enterprise/status", s.status)
	r.POST("/api/enterprise/discover", func(c *gin.Context) {
		var in struct {
			URL string `json:"serverUrl"`
		}
		if c.ShouldBindJSON(&in) != nil {
			errorJSON(c, 400, errors.New("请输入服务器地址"))
			return
		}
		d, err := s.discover(c.Request.Context(), in.URL)
		if err != nil {
			errorJSON(c, 400, err)
			return
		}
		c.JSON(200, d)
	})
	r.POST("/api/enterprise/login", s.login)
	r.GET("/api/enterprise/login-url", func(c *gin.Context) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.pending == nil || time.Now().After(s.pending.Expires) {
			c.JSON(404, gin.H{"error": "登录已过期"})
			return
		}
		c.JSON(200, gin.H{"url": s.pending.URL})
	})
	r.POST("/api/enterprise/poll", s.poll)
	r.POST("/api/enterprise/complete", s.complete)
	r.POST("/api/enterprise/cancel", s.cancel)
	r.POST("/api/enterprise/disconnect", s.disconnect)
	r.Any("/enterprise/*path", s.proxy)
}
func (s *Client) status(c *gin.Context) {
	s.mu.Lock()
	row, err := s.record()
	var cred connectors.Credential
	var credentialErr error
	if err == nil && row.Active {
		cred, credentialErr = s.vault.Get(vaultID)
	}
	s.mu.Unlock()
	if err != nil {
		errorJSON(c, 500, errors.New("无法读取空间设置"))
		return
	}
	if !row.Active {
		c.JSON(200, gin.H{"mode": "personal", "connection": row})
		return
	}
	if credentialErr != nil || cred.ClientID != row.binding() {
		c.JSON(200, gin.H{"mode": "enterprise", "connection": row, "needsLogin": true})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	err = s.checkSession(ctx, row, cred.APIKey)
	if errors.Is(err, errSessionInvalid) {
		c.JSON(200, gin.H{"mode": "enterprise", "connection": row, "needsLogin": true})
		return
	}
	if err != nil {
		c.JSON(200, gin.H{"mode": "enterprise", "connection": row, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"mode": "enterprise", "connection": row})
}
func (s *Client) login(c *gin.Context) {
	var in struct {
		URL string `json:"serverUrl"`
	}
	if c.ShouldBindJSON(&in) != nil {
		errorJSON(c, 400, errors.New("请输入服务器地址"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.discover(c.Request.Context(), in.URL)
	if err != nil {
		errorJSON(c, 400, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	status, raw, err := s.request(ctx, "POST", d.ServerURL+"/auth/attempts", "", []byte(`{}`))
	var a loginAttempt
	if err != nil {
		errorJSON(c, 502, err)
		return
	}
	if status != 201 || json.Unmarshal(raw, &a) != nil || len(a.ID) != 43 || len(a.Secret) != 43 || a.URL != d.ServerURL+"/auth/browser?ticket="+a.ID || time.Now().After(a.Expires) || time.Until(a.Expires) > 10*time.Minute {
		errorJSON(c, 502, errors.New("无法开始公司登录，请联系管理员检查服务"))
		return
	}
	a.ServerURL = d.ServerURL
	s.pending = &a
	c.JSON(200, gin.H{"id": a.ID, "url": a.URL, "organization": d.Name, "expiresAt": a.Expires})
}

// Caller must identify the attempt so a stale panel cannot complete a newer login.
func (s *Client) pendingFor(c *gin.Context) *loginAttempt {
	var in struct {
		ID string `json:"id"`
	}
	if c.ShouldBindJSON(&in) != nil || s.pending == nil || in.ID != s.pending.ID || time.Now().After(s.pending.Expires) {
		errorJSON(c, 401, errors.New("登录已过期，请重试"))
		return nil
	}
	return s.pending
}
func (s *Client) poll(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.pendingFor(c)
	if a == nil {
		return
	}
	if a.Token != "" {
		c.JSON(200, gin.H{"status": "authorized", "connection": a.Connection})
		return
	}
	body, _ := json.Marshal(map[string]string{"id": a.ID, "pollSecret": a.Secret})
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	status, raw, err := s.request(ctx, "POST", a.ServerURL+"/auth/poll", "", body)
	if err != nil {
		errorJSON(c, 502, err)
		return
	}
	if status == 202 {
		c.JSON(202, gin.H{"status": "pending"})
		return
	}
	if status >= 500 || status == http.StatusTooManyRequests {
		errorJSON(c, 503, errors.New("公司服务暂不可用，请稍后重试"))
		return
	}
	var result struct {
		identity
		Token string `json:"token"`
	}
	if status != 200 || json.Unmarshal(raw, &result) != nil || len(result.Token) != 43 || result.User.ID == "" || result.WorkspaceID == "" {
		s.pending = nil
		errorJSON(c, 401, errors.New("登录未完成，请确认账号已获公司授权后重试"))
		return
	}
	a.Token = result.Token
	a.Connection = Record{ID: 1, Active: true, ServerURL: a.ServerURL, Organization: result.Organization, UserID: result.User.ID, UserName: result.User.Name, WorkspaceID: result.WorkspaceID}
	c.JSON(200, gin.H{"status": "authorized", "connection": a.Connection})
}
func (s *Client) complete(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.pendingFor(c)
	if a == nil {
		return
	}
	if a.Token == "" {
		errorJSON(c, 409, errors.New("请先完成浏览器授权"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	err := s.checkSession(ctx, a.Connection, a.Token)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, errSessionInvalid) {
			status = http.StatusUnauthorized
		}
		errorJSON(c, status, err)
		return
	}
	previous, previousErr := s.vault.Get(vaultID)
	// Bind the vault entry to the full identity. If the process crashes between
	// vault and DB writes, an older record cannot send requests as the new user.
	if err = s.vault.Set(vaultID, connectors.Credential{APIKey: a.Token, ClientID: a.Connection.binding()}); err != nil {
		errorJSON(c, 500, errors.New("系统凭据库不可用，无法保存公司登录"))
		return
	}
	if err = s.db.Save(&a.Connection).Error; err != nil {
		if previousErr == nil {
			_ = s.vault.Set(vaultID, previous)
		} else {
			_ = s.vault.Delete(vaultID)
		}
		errorJSON(c, 500, errors.New("无法保存公司空间"))
		return
	}
	s.pending = nil
	c.JSON(200, gin.H{"mode": "enterprise", "connection": a.Connection})
}
func (s *Client) cancel(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var in struct {
		ID string `json:"id"`
	}
	if c.ShouldBindJSON(&in) != nil {
		c.Status(400)
		return
	}
	if a := s.pending; a != nil && a.ID == in.ID {
		s.pending = nil
		// Cancellation never changes the active space, even if a poll just finished.
		if a.Token != "" {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
			defer cancel()
			_, _, _ = s.request(ctx, "POST", a.ServerURL+"/api/session/logout", a.Token, []byte(`{}`))
		}
	}
	c.Status(204)
}
func (s *Client) disconnect(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, err := s.record()
	if err != nil {
		errorJSON(c, 500, err)
		return
	}
	revoked := !row.Active
	if cred, e := s.vault.Get(vaultID); e == nil && cred.ClientID == row.binding() && row.ServerURL != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		status, _, e := s.request(ctx, "POST", row.ServerURL+"/api/session/logout", cred.APIKey, []byte(`{}`))
		cancel()
		revoked = e == nil && (status == 204 || status == 401)
	}
	row.Active = false
	row.ID = 1
	if err = s.db.Save(&row).Error; err != nil {
		errorJSON(c, 500, errors.New("无法退出公司空间，请重试"))
		return
	}
	deleted := s.vault.Delete(vaultID) == nil
	s.pending = nil
	c.JSON(200, gin.H{"mode": "personal", "revoked": revoked, "credentialDeleted": deleted})
}
func (s *Client) proxy(c *gin.Context) {
	path := c.Param("path")
	if !strings.HasPrefix(path, "/api/") || strings.Contains(path, "..") || strings.ContainsAny(path, "\\?#") {
		c.AbortWithStatus(404)
		return
	}
	s.mu.Lock()
	row, err := s.record()
	var cred connectors.Credential
	var credentialErr error
	if err == nil && row.Active {
		cred, credentialErr = s.vault.Get(vaultID)
	}
	s.mu.Unlock()
	if err != nil {
		errorJSON(c, 503, errors.New("无法读取空间设置，请稍后重试"))
		return
	}
	if !row.Active {
		errorJSON(c, 401, errors.New("请先登录公司空间"))
		return
	}
	if credentialErr != nil || cred.ClientID != row.binding() {
		errorJSON(c, 401, errors.New("公司登录已失效，请重新登录"))
		return
	}
	if c.GetHeader("X-Soren-Workspace") != row.binding() {
		errorJSON(c, 409, errors.New("使用空间已切换，请刷新后重试"))
		return
	}
	target, err := url.Parse(row.ServerURL + path)
	if err != nil {
		c.AbortWithStatus(400)
		return
	}
	target.RawQuery = c.Request.URL.RawQuery
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target.String(), c.Request.Body)
	if err != nil {
		c.AbortWithStatus(400)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cred.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bot-ID", c.GetHeader("X-Bot-ID"))
	res, err := s.http.Do(req)
	if err != nil {
		errorJSON(c, 502, errors.New("公司服务器暂不可达，请检查网络"))
		return
	}
	defer res.Body.Close()
	// Only the authenticated attachment endpoint may stream binary data.
	if c.Request.Method == "GET" && attachmentPath.MatchString(path) && res.StatusCode == 200 {
		if res.ContentLength < 0 || res.ContentLength > 200<<20 {
			errorJSON(c, 502, errors.New("成果文件过大或长度未知"))
			return
		}
		c.Header("Content-Disposition", res.Header.Get("Content-Disposition"))
		c.Header("X-Content-Type-Options", "nosniff")
		c.DataFromReader(200, res.ContentLength, "application/octet-stream", io.LimitReader(res.Body, res.ContentLength), nil)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	if err != nil || len(raw) > 4<<20 || (len(raw) > 0 && !json.Valid(raw)) || res.StatusCode >= 300 && res.StatusCode < 400 {
		errorJSON(c, 502, errors.New("公司服务器响应无效或过大"))
		return
	}
	c.Data(res.StatusCode, "application/json", raw)
}

var attachmentPath = regexp.MustCompile(`^/api/runs/[a-f0-9-]{36}/files/[a-f0-9-]{36}$`)
