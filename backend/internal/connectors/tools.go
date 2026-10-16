package connectors

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

type Artifact struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	RunID     string    `gorm:"index" json:"runId"`
	BotID     string    `gorm:"index" json:"botId"`
	Title     string    `json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func sortedKeys(m map[string]binding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func (s *Service) execute(ctx context.Context, b binding, bot, run string, raw []byte) (string, error) {
	args := map[string]string{}
	if b.Connection.Kind != "mcp" {
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", errors.New("参数必须是文本字段")
		}
	}
	for _, value := range args {
		if len(value) > 90000 {
			return "", errors.New("内容过长")
		}
	}
	if b.Connection.ID == "" {
		switch b.Action {
		case "workspace_create":
			if strings.TrimSpace(args["title"]) == "" || strings.TrimSpace(args["content"]) == "" || len(args["title"]) > 200 {
				return "", errors.New("请填写标题和正文")
			}
			a := Artifact{ID: uuid.NewString(), RunID: run, BotID: bot, Title: args["title"], Content: args["content"]}
			if e := s.DB.Create(&a).Error; e != nil {
				return "", e
			}
			return jsonText(map[string]any{"id": a.ID, "title": a.Title, "status": "saved"}), nil
		case "workspace_list":
			var a []Artifact
			e := s.DB.Select("id, title, created_at").Where("bot_id = ?", bot).Order("created_at DESC").Limit(30).Find(&a).Error
			return jsonText(a), e
		case "workspace_read":
			var a Artifact
			e := s.DB.First(&a, "id = ? AND bot_id = ?", args["id"], bot).Error
			return jsonText(a), e
		}
	}
	credential, e := s.Vault.Get(b.Connection.ID)
	if e != nil {
		return "", errors.New("无法读取连接凭据，请重新连接")
	}
	if k, ok := kinds[b.Connection.Kind]; ok {
		return s.executeKind(ctx, k, b.Action, credential.APIKey, args)
	}
	switch b.Connection.Kind {
	case "gmail":
		addr, e := mail.ParseAddress(args["to"])
		if e != nil || strings.ContainsAny(args["subject"], "\r\n") || args["body"] == "" {
			return "", errors.New("收件人、主题或正文无效")
		}
		// MIME encoding prevents recipient/subject input from becoming new headers.
		message := fmt.Sprintf("To: %s\r\nSubject: =?UTF-8?B?%s?=\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s", addr.String(), base64.StdEncoding.EncodeToString([]byte(args["subject"])), base64.StdEncoding.EncodeToString([]byte(args["body"])))
		body := map[string]any{"raw": base64.RawURLEncoding.EncodeToString([]byte(message))}
		endpoint := "/users/me/messages/send"
		if b.Action == "draft" {
			body = map[string]any{"message": body}
			endpoint = "/users/me/drafts"
		}
		token, e := s.googleToken(ctx, b.Connection.ID, credential)
		if e != nil {
			return "", e
		}
		return s.request(ctx, "POST", s.GoogleAPI+endpoint, token, body, nil)
	case "notion":
		headers := map[string]string{"Notion-Version": "2022-06-28"}
		switch b.Action {
		case "search":
			return s.request(ctx, "POST", s.NotionAPI+"/search", credential.APIKey, map[string]any{"query": args["query"], "page_size": 30}, headers)
		case "read":
			id, e := uuid.Parse(args["pageId"])
			if e != nil {
				return "", errors.New("页面 ID 无效")
			}
			q := url.Values{"page_size": {"100"}}
			if args["cursor"] != "" {
				q.Set("start_cursor", args["cursor"])
			}
			return s.request(ctx, "GET", s.NotionAPI+"/blocks/"+id.String()+"/children?"+q.Encode(), credential.APIKey, nil, headers)
		case "create":
			id, e := uuid.Parse(args["parentId"])
			if e != nil || args["title"] == "" || len([]rune(args["title"])) > 200 {
				return "", errors.New("父页面或标题无效")
			}
			children := []any{}
			text := []rune(args["content"])
			for len(text) > 0 {
				n := 1800
				if n > len(text) {
					n = len(text)
				}
				children = append(children, map[string]any{"object": "block", "type": "paragraph", "paragraph": map[string]any{"rich_text": []any{map[string]any{"type": "text", "text": map[string]any{"content": string(text[:n])}}}}})
				text = text[n:]
			}
			return s.request(ctx, "POST", s.NotionAPI+"/pages", credential.APIKey, map[string]any{"parent": map[string]any{"page_id": id.String()}, "properties": map[string]any{"title": map[string]any{"type": "title", "title": []any{map[string]any{"type": "text", "text": map[string]any{"content": args["title"]}}}}}, "children": children}, headers)
		}
	case "mcp":
		session, e := s.mcpSession(ctx, b.Connection.Endpoint, credential.APIKey)
		if e != nil {
			return "", e
		}
		defer session.Close()
		var arguments map[string]any
		if e = json.Unmarshal(raw, &arguments); e != nil {
			return "", e
		}
		result, e := session.CallTool(ctx, &mcp.CallToolParams{Name: b.Action, Arguments: arguments})
		if e != nil {
			return "", e
		}
		if result.IsError {
			return "", errors.New("MCP 工具返回失败")
		}
		return jsonText(result), nil
	}
	return "", errors.New("不支持的工具")
}
func jsonText(v any) string { b, _ := json.Marshal(v); return string(b) }
func (s *Service) request(ctx context.Context, method, endpoint, token string, body any, headers map[string]string) (string, error) {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return "", e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if e != nil {
		return "", e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, e := s.HTTP.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	data, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return "", e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("服务返回状态 %d", res.StatusCode)
	}
	return string(data), nil
}
func (s *Service) googleToken(ctx context.Context, id string, c Credential) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Reload after acquiring the lock so simultaneous calls cannot rotate a stale token.
	var e error
	c, e = s.Vault.Get(id)
	if e != nil {
		return "", e
	}
	if c.Token == nil {
		return "", errors.New("请重新授权 Gmail")
	}
	config := googleConfig(c, "")
	token, e := config.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, s.HTTP), c.Token).Token()
	if e != nil {
		return "", errors.New("Gmail 授权已失效，请重新连接")
	}
	if token.AccessToken != c.Token.AccessToken {
		c.Token = token
		if e = s.Vault.Set(id, c); e != nil {
			return "", e
		}
	}
	return token.AccessToken, nil
}

// Only explicitly configured HTTPS MCP endpoints are accepted. Resolve again at
// dial time and reject local/link-local targets and redirects carrying credentials.
func validEndpoint(endpoint string) error {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("MCP 地址必须是无查询参数的 HTTPS URL")
	}
	return nil
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	if t.token != "" {
		clone.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.base.RoundTrip(clone)
}

// Resolve and pin the public target before selecting the administrator's proxy.
// CONNECT uses the validated IP; TLS still verifies the original hostname. This
// allows enterprise egress without letting proxy-side DNS bypass SSRF checks.
type publicRoundTripper struct{}

func publicTransport() http.RoundTripper { return publicRoundTripper{} }
func (publicRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.User != nil {
		return nil, errors.New("远程服务必须使用 HTTPS")
	}
	host := r.URL.Hostname()
	port := r.URL.Port()
	if port == "" {
		port = "443"
	}
	ips, e := net.DefaultResolver.LookupIPAddr(r.Context(), host)
	if e != nil {
		return nil, e
	}
	if len(ips) == 0 {
		return nil, errors.New("服务地址无法解析")
	}
	for _, ip := range ips {
		if ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsUnspecified() || ip.IP.IsMulticast() {
			return nil, errors.New("远程服务不允许访问本机或内网地址")
		}
	}
	proxy, e := http.ProxyFromEnvironment(r)
	if e != nil {
		return nil, e
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxy)
	transport.DisableKeepAlives = true
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
	transport.TLSClientConfig = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	clone := r.Clone(r.Context())
	target := *r.URL
	target.Host = net.JoinHostPort(ips[0].IP.String(), port)
	clone.URL = &target
	clone.Host = r.URL.Host
	return transport.RoundTrip(clone)
}
func (s *Service) mcpSession(ctx context.Context, endpoint, token string) (*mcp.ClientSession, error) {
	if e := validEndpoint(endpoint); e != nil {
		return nil, e
	}
	transport := s.MCPTransport
	if transport == nil {
		transport = publicTransport()
	}
	client := &http.Client{Transport: bearerTransport{transport, token}, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("MCP 重定向未获授权") }}
	return mcp.NewClient(&mcp.Implementation{Name: "Soren", Version: "0.3.0"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: client, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
}
func (s *Service) Discover(ctx context.Context, endpoint, token string) ([]RemoteTool, error) {
	session, e := s.mcpSession(ctx, endpoint, token)
	if e != nil {
		return nil, errors.New("MCP 连接失败；请检查地址及访问令牌")
	}
	defer session.Close()
	out := []RemoteTool{}
	for t, e := range session.Tools(ctx, nil) {
		if e != nil {
			return nil, e
		}
		schema, e := json.Marshal(t.InputSchema)
		if e != nil {
			return nil, e
		}
		out = append(out, RemoteTool{t.Name, t.Description, schema})
		if len(out) >= 32 {
			break
		}
	}
	return out, nil
}
