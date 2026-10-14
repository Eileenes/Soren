package templatesclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/botconfig"
	"github.com/soren/backend/internal/bottemplate"
	"github.com/soren/backend/internal/connectors"
	"gorm.io/gorm"
)

type Settings struct {
	ID           int `gorm:"primaryKey"`
	URL          string
	CredentialID string
}
type SettingsView struct {
	URL        string `json:"url"`
	Configured bool   `json:"configured"`
}
type SettingsInput struct {
	URL        string `json:"url"`
	Token      string `json:"token"`
	ClearToken bool   `json:"clearToken"`
}
type Preview struct {
	ID            string                `gorm:"primaryKey" json:"previewId"`
	Service       string                `json:"service"`
	Published     bottemplate.Published `gorm:"serializer:json;type:text" json:"published"`
	ExpiresAt     time.Time             `json:"expiresAt"`
	ExistingBotID string                `gorm:"-" json:"existingBotId,omitempty"`
	BotID         string                `json:"-"`
	ImportName    string                `json:"-"`
}
type Client struct {
	DB     *gorm.DB
	Agents *agent.Service
	Vault  connectors.Vault
	HTTP   *http.Client
	mu     sync.Mutex
}

func New(agents *agent.Service, vault connectors.Vault) (*Client, error) {
	c := &Client{DB: agents.DB, Agents: agents, Vault: vault, HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if e := c.DB.AutoMigrate(&Settings{}, &Preview{}); e != nil {
		return nil, e
	}
	return c, nil
}
func (c *Client) settings() (Settings, error) {
	var s Settings
	e := c.DB.First(&s, "id = 1").Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return Settings{ID: 1}, nil
	}
	return s, e
}
func (c *Client) Get() (SettingsView, error) {
	s, e := c.settings()
	return SettingsView{s.URL, s.CredentialID != ""}, e
}
func (c *Client) Save(input SettingsInput) (SettingsView, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	base := ""
	var e error
	if strings.TrimSpace(input.URL) != "" {
		base, e = bottemplate.ServiceURL(input.URL)
		if e != nil {
			return SettingsView{}, e
		}
	}
	old, e := c.settings()
	if e != nil {
		return SettingsView{}, e
	}
	next := Settings{ID: 1, URL: base, CredentialID: old.CredentialID}
	token := strings.TrimSpace(input.Token)
	if input.ClearToken || old.URL != base || base == "" {
		next.CredentialID = ""
	}
	if token != "" && !input.ClearToken && base != "" {
		if len(token) < 32 || len(token) > 256 || strings.ContainsAny(token, " \r\n\t") {
			return SettingsView{}, errors.New("发布凭据格式无效")
		}
		next.CredentialID = "bot-templates-" + uuid.NewString()
		if e = c.Vault.Set(next.CredentialID, connectors.Credential{APIKey: token}); e != nil {
			return SettingsView{}, errors.New("无法将发布凭据写入系统钥匙串")
		}
	}
	if e = c.DB.Save(&next).Error; e != nil {
		if next.CredentialID != "" && next.CredentialID != old.CredentialID {
			_ = c.Vault.Delete(next.CredentialID)
		}
		return SettingsView{}, e
	}
	if old.CredentialID != "" && old.CredentialID != next.CredentialID {
		_ = c.Vault.Delete(old.CredentialID)
	}
	return SettingsView{base, next.CredentialID != ""}, nil
}
func (c *Client) call(ctx context.Context, s Settings, method, path string, body, out any, auth bool) error {
	if s.URL == "" {
		return errors.New("请先在设置中填写 Bot 模板服务地址")
	}
	var reader io.Reader
	if body != nil {
		data, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(data)
	}
	req, e := http.NewRequestWithContext(ctx, method, s.URL+path, reader)
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		if s.CredentialID == "" {
			return errors.New("请先在设置中添加你的发布凭据")
		}
		secret, e := c.Vault.Get(s.CredentialID)
		if e != nil {
			return errors.New("无法读取发布凭据，请重新配置")
		}
		req.Header.Set("Authorization", "Bearer "+secret.APIKey)
	}
	res, e := c.HTTP.Do(req)
	if e != nil {
		return errors.New("无法连接模板服务，请检查地址或稍后重试")
	}
	defer res.Body.Close()
	data, e := io.ReadAll(io.LimitReader(res.Body, bottemplate.MaxBytes+1))
	if e != nil || len(data) > bottemplate.MaxBytes {
		return errors.New("模板服务响应超过限制或读取失败")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		switch res.StatusCode {
		case 401, 403:
			return errors.New("发布凭据无效、已撤销或无权管理此模板")
		case 404:
			return errors.New("模板不存在、已停用或已删除")
		case 409:
			return errors.New("模板状态或请求内容已变更，请刷新后重试")
		case 429:
			return errors.New("请求过于频繁，请稍后重试")
		default:
			return errors.New("模板服务未能完成请求")
		}
	}
	if out != nil {
		return bottemplate.Decode(data, out)
	}
	return nil
}
func (c *Client) Identity(ctx context.Context) (map[string]string, error) {
	s, e := c.settings()
	if e != nil {
		return nil, e
	}
	var out map[string]string
	e = c.call(ctx, s, "GET", "/api/me", nil, &out, true)
	return out, e
}
func (c *Client) List(ctx context.Context) ([]bottemplate.Published, error) {
	s, e := c.settings()
	if e != nil {
		return nil, e
	}
	out := []bottemplate.Published{}
	after := ""
	for {
		page := []bottemplate.Published{}
		if e = c.call(ctx, s, "GET", "/api/templates?after="+url.QueryEscape(after), nil, &page, true); e != nil {
			return nil, e
		}
		out = append(out, page...)
		if len(page) < 50 {
			return out, nil
		}
		next := page[len(page)-1].ID
		if next <= after || !bottemplate.ValidID(next) {
			return nil, errors.New("模板服务分页无效")
		}
		after = next
	}
}
func (c *Client) Publish(ctx context.Context, id string, input bottemplate.PublishRequest) (bottemplate.Published, error) {
	var out bottemplate.Published
	if e := input.Config.Validate(); e != nil {
		return out, e
	}
	if !bottemplate.ValidID(input.RequestID) || (id != "" && !bottemplate.ValidID(id)) {
		return out, errors.New("请求标识无效")
	}
	s, e := c.settings()
	if e != nil {
		return out, e
	}
	path := "/api/templates"
	if id != "" {
		path += "/" + id + "/versions"
	}
	e = c.call(ctx, s, "POST", path, input, &out, true)
	return out, e
}
func (c *Client) Manage(ctx context.Context, id string, disabled *bool) error {
	if !bottemplate.ValidID(id) {
		return errors.New("模板标识无效")
	}
	s, e := c.settings()
	if e != nil {
		return e
	}
	if disabled == nil {
		return c.call(ctx, s, "DELETE", "/api/templates/"+id, nil, nil, true)
	}
	return c.call(ctx, s, "PATCH", "/api/templates/"+id, map[string]bool{"disabled": *disabled}, nil, true)
}
func Export(b *agent.Bot) bottemplate.Config {
	keys := []string{}
	if b.Policy != nil {
		for key, value := range b.Policy.Actions {
			if value != "deny" {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	// The API only ever returns this explicit portable allowlist.
	return bottemplate.Config{Format: bottemplate.Format, SchemaVersion: 1, Name: b.Name, Avatar: b.Avatar, Instructions: b.Instructions, Definition: b.Definition, PolicyTemplate: b.Policy, MemoryEnabled: b.MemoryEnabled, RequiredCapabilities: keys}
}
func parseLink(base, raw string) (string, int, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || len(raw) > 2048 {
		return "", 0, errors.New("模板链接无效")
	}
	var id, revision string
	if u.User != nil || u.Fragment != "" {
		return "", 0, errors.New("模板链接无效")
	}
	if u.Scheme == "soren" {
		if u.Host != "templates" || u.Path != "/import" {
			return "", 0, errors.New("不是模板导入链接")
		}
		q, e := url.ParseQuery(u.RawQuery)
		if e != nil {
			return "", 0, errors.New("模板链接参数无效")
		}
		for key, value := range q {
			if (key != "id" && key != "revision" && key != "service") || len(value) != 1 {
				return "", 0, errors.New("模板链接参数无效")
			}
		}
		if q.Get("service") != base {
			return "", 0, errors.New("链接来自另一模板服务，请先核对并修改设置中的服务地址")
		}
		id, revision = q.Get("id"), q.Get("revision")
	} else {
		if u.Scheme+"://"+u.Host != base || u.RawQuery != "" {
			return "", 0, errors.New("链接与已配置的模板服务不一致，请先核对服务地址")
		}
		parts := strings.Split(u.Path, "/")
		if len(parts) != 5 || parts[1] != "t" || parts[3] != "v" {
			return "", 0, errors.New("请粘贴完整的模板版本链接")
		}
		id, revision = parts[2], parts[4]
	}
	n, e := strconv.Atoi(revision)
	if e != nil || n < 1 || !bottemplate.ValidID(id) {
		return "", 0, errors.New("模板或版本标识无效")
	}
	return id, n, nil
}
func (c *Client) fetch(ctx context.Context, s Settings, id string, revision int) (bottemplate.Published, error) {
	var out bottemplate.Published
	e := c.call(ctx, s, "GET", fmt.Sprintf("/api/templates/%s/versions/%d", id, revision), nil, &out, false)
	if e != nil {
		return out, e
	}
	if e = out.Config.Validate(); e != nil {
		return out, e
	}
	if out.ID != id || out.Revision != revision || out.Disabled || out.Digest != bottemplate.Digest(out.Config) {
		return out, errors.New("模板版本或内容校验失败")
	}
	out.URL = fmt.Sprintf("%s/t/%s/v/%d", s.URL, id, revision)
	return out, nil
}
func (c *Client) Preview(ctx context.Context, link string) (Preview, error) {
	s, e := c.settings()
	if e != nil {
		return Preview{}, e
	}
	if s.URL == "" {
		return Preview{}, errors.New("请先配置 Bot 模板服务地址，再打开分享链接")
	}
	id, n, e := parseLink(s.URL, link)
	if e != nil {
		return Preview{}, e
	}
	out, e := c.fetch(ctx, s, id, n)
	if e != nil {
		return Preview{}, e
	}
	p := Preview{ID: uuid.NewString(), Service: s.URL, Published: out, ExpiresAt: time.Now().Add(30 * time.Minute)}
	if e = c.DB.Create(&p).Error; e != nil {
		return p, e
	}
	var bots []agent.Bot
	if e = c.DB.Find(&bots).Error; e != nil {
		return p, e
	}
	for _, b := range bots {
		src := b.TemplateSource
		if src != nil && src.Service == s.URL && src.ID == id && src.Revision == n {
			p.ExistingBotID = b.ID
			break
		}
	}
	_ = c.DB.Where("expires_at < ? AND bot_id = ?", time.Now(), "").Delete(&Preview{}).Error
	return p, nil
}
func (c *Client) Import(ctx context.Context, previewID, name string) (*agent.Bot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var p Preview
	if e := c.DB.First(&p, "id = ?", previewID).Error; e != nil {
		return nil, errors.New("导入预览已失效，请重新打开链接")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = p.Published.Config.Name
	}
	if p.BotID != "" {
		if p.ImportName != name {
			return nil, errors.New("此预览已用于另一个名称")
		}
		return c.Agents.GetBot(p.BotID)
	}
	if time.Now().After(p.ExpiresAt) {
		return nil, errors.New("预览已过期，请重新加载模板")
	}
	s, e := c.settings()
	if e != nil {
		return nil, e
	}
	if s.URL != p.Service {
		return nil, errors.New("模板服务已切换，请重新预览")
	}
	latest, e := c.fetch(ctx, s, p.Published.ID, p.Published.Revision)
	if e != nil {
		return nil, e
	}
	if latest.Digest != p.Published.Digest {
		return nil, errors.New("预览内容已变更，请重新预览")
	}
	config := p.Published.Config
	avatar := config.Avatar
	if !agent.Contains(agent.Avatars, avatar) {
		avatar = "robo"
	}
	policy := &botconfig.Policy{Actions: map[string]string{}}
	for _, key := range config.RequiredCapabilities {
		policy.Actions[key] = "deny"
	}
	if config.PolicyTemplate != nil {
		for key := range config.PolicyTemplate.Actions {
			policy.Actions[key] = "deny"
		}
	}
	b := &agent.Bot{Name: name, Avatar: avatar, Instructions: config.Instructions, Definition: config.Definition, Policy: policy, MemoryEnabled: config.MemoryEnabled, CollaborationEnabled: false, CollaborationAll: false, Collaborators: []string{}, TemplateSource: &agent.TemplateSource{Service: s.URL, ID: latest.ID, Revision: latest.Revision, Digest: latest.Digest}}
	e = c.DB.Transaction(func(tx *gorm.DB) error {
		local := &agent.Service{DB: tx}
		if e := local.SaveBot(b, true); e != nil {
			return e
		}
		return tx.Model(&Preview{}).Where("id = ?", p.ID).Updates(map[string]any{"bot_id": b.ID, "import_name": name}).Error
	})
	return b, e
}
