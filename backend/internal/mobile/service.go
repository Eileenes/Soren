// Package mobile owns device trust. The relay is only a ciphertext transport.
package mobile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/relay"
	"golang.org/x/crypto/nacl/box"
	"gorm.io/gorm"
)

// Release builds inject product-operated endpoints. Empty means not deployed.
var DefaultRelayURL, DefaultMobileURL string
var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)
var runPath = regexp.MustCompile(`^/api/runs/[a-zA-Z0-9_-]+/(cancel|resume|tools|artifacts)$`)
var decisionPath = regexp.MustCompile(`^/api/tool-invocations/[a-zA-Z0-9_-]+/decision$`)

type Device struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	AddedAt int64  `json:"addedAt"`
}
type identity struct {
	Public    [32]byte          `json:"public"`
	Private   [32]byte          `json:"private"`
	HostToken string            `json:"hostToken"`
	Enabled   bool              `json:"enabled"`
	Devices   map[string]Device `json:"devices"`
}
type Envelope struct {
	Key   string `json:"key"`
	Nonce string `json:"nonce"`
	Box   string `json:"box"`
}
type Request struct {
	ID       string          `json:"id"`
	Deadline int64           `json:"deadline"`
	Kind     string          `json:"kind"`
	Secret   string          `json:"secret,omitempty"`
	Name     string          `json:"name,omitempty"`
	Method   string          `json:"method,omitempty"`
	Path     string          `json:"path,omitempty"`
	BotID    string          `json:"botId,omitempty"`
	Body     json.RawMessage `json:"body,omitempty"`
}
type Response struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}
type receipt struct {
	ID       string `gorm:"primaryKey"`
	Digest   string
	Deadline int64 `gorm:"index"`
	Status   int
	Body     []byte
}

func (receipt) TableName() string { return "mobile_receipts" }

type pairing struct {
	Secret  string
	Expires int64
	Pending *Device
	Code    string
}
type Service struct {
	mu                  sync.Mutex
	state               identity
	path                string
	db                  *gorm.DB
	handler             http.Handler
	relayURL, mobileURL string
	pair                *pairing
	online              bool
	client              *http.Client
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func decode(s string, n int) ([]byte, error) {
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil || (n > 0 && len(b) != n) {
		return nil, errors.New("invalid encoding")
	}
	return b, nil
}
func endpoint(raw string, local bool) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("invalid endpoint")
	}
	if u.Scheme != "https" && !(local && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")) {
		return "", errors.New("HTTPS required")
	}
	return strings.TrimRight(raw, "/"), nil
}
func New(db *gorm.DB, dir string, handler http.Handler, relayURL, mobileURL string, allowLocal bool) (*Service, error) {
	s := &Service{db: db, path: filepath.Join(dir, "mobile-identity.json"), handler: handler, client: &http.Client{Timeout: 35 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if relayURL != "" {
		var e error
		s.relayURL, e = endpoint(relayURL, allowLocal)
		if e != nil {
			return nil, e
		}
		s.mobileURL, e = endpoint(mobileURL, allowLocal)
		if e != nil {
			return nil, e
		}
	}
	if e := db.AutoMigrate(&receipt{}); e != nil {
		return nil, e
	}
	raw, e := os.ReadFile(s.path)
	if e == nil {
		if json.Unmarshal(raw, &s.state) != nil || s.state.HostToken == "" || s.state.Devices == nil {
			return nil, errors.New("invalid mobile identity")
		}
	} else if os.IsNotExist(e) {
		pub, priv, e := box.GenerateKey(rand.Reader)
		if e != nil {
			return nil, e
		}
		s.state = identity{Public: *pub, Private: *priv, HostToken: relay.RandomID(), Devices: map[string]Device{}}
		if e = s.save(); e != nil {
			return nil, e
		}
	} else {
		return nil, e
	}
	return s, nil
}
func (s *Service) save() error {
	raw, e := json.Marshal(s.state)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		return e
	}
	tmp := s.path + ".tmp"
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, s.path)
}
func response(id string, status int, v any) Response {
	raw, _ := json.Marshal(v)
	return Response{ID: id, Status: status, Body: raw}
}
func failure(id string, status int, msg string) Response {
	return response(id, status, map[string]string{"error": msg})
}
func code(pub, host []byte) string {
	sum := sha256.Sum256(append(append([]byte{}, host...), pub...))
	return fmt.Sprintf("%06d", (uint32(sum[0])<<16|uint32(sum[1])<<8|uint32(sum[2]))%1000000)
}
func (s *Service) Handle(raw json.RawMessage) json.RawMessage {
	var env Envelope
	if len(raw) > relay.MaxEnvelope || json.Unmarshal(raw, &env) != nil {
		return json.RawMessage(`{}`)
	}
	pk, e := decode(env.Key, 32)
	if e != nil {
		return json.RawMessage(`{}`)
	}
	nonce, e := decode(env.Nonce, 24)
	if e != nil {
		return json.RawMessage(`{}`)
	}
	cipher, e := decode(env.Box, 0)
	if e != nil || len(cipher) > 256<<10 {
		return json.RawMessage(`{}`)
	}
	var public [32]byte
	copy(public[:], pk)
	var n [24]byte
	copy(n[:], nonce)
	plain, ok := box.Open(nil, cipher, &n, &public, &s.state.Private)
	if !ok {
		return json.RawMessage(`{}`)
	}
	var req Request
	var res Response
	if json.Unmarshal(plain, &req) != nil {
		res = failure("", 400, "请求无效")
	} else {
		res = s.dispatch(env.Key, pk, req)
	}
	payload, _ := json.Marshal(res)
	if _, e = rand.Read(n[:]); e != nil {
		return json.RawMessage(`{}`)
	}
	out, _ := json.Marshal(Envelope{Key: b64(s.state.Public[:]), Nonce: b64(n[:]), Box: b64(box.Seal(nil, payload, &n, &public, &s.state.Private))})
	return out
}
func (s *Service) dispatch(key string, pk []byte, req Request) Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if !s.state.Enabled {
		return failure(req.ID, 403, "手机连接已关闭")
	}
	if !idPattern.MatchString(req.ID) || req.Deadline < now || req.Deadline > now+180 {
		return failure(req.ID, 400, "请求已过期，请刷新")
	}
	if req.Kind == "pair" {
		if _, ok := s.state.Devices[key]; ok {
			return response(req.ID, 200, map[string]string{"state": "paired"})
		}
		p := s.pair
		if p == nil || p.Expires < now || subtle.ConstantTimeCompare([]byte(req.Secret), []byte(p.Secret)) != 1 {
			return failure(req.ID, 403, "二维码已失效，请重新扫码")
		}
		if p.Pending != nil && p.Pending.Key != key {
			return failure(req.ID, 409, "已有设备等待确认，请重新生成二维码")
		}
		if p.Pending == nil {
			name := strings.TrimSpace(req.Name)
			if name == "" || len([]rune(name)) > 60 {
				return failure(req.ID, 400, "设备名称无效")
			}
			p.Pending = &Device{Key: key, Name: name, AddedAt: now}
			p.Code = code(pk, s.state.Public[:])
		}
		return response(req.ID, 202, map[string]string{"state": "pending", "code": p.Code})
	}
	if _, ok := s.state.Devices[key]; !ok {
		return failure(req.ID, 403, "设备未关联或已被撤销")
	}
	if req.Kind != "api" || !allowed(req.Method, req.Path) || !idPattern.MatchString(req.BotID) || len(req.Body) > 64<<10 {
		return failure(req.ID, 403, "此操作请在 Mac 上进行")
	}
	write := req.Method != "GET"
	digestRaw, _ := json.Marshal(req)
	digest := sha256.Sum256(digestRaw)
	receiptID := key + ":" + req.ID
	if write {
		var old receipt
		err := s.db.First(&old, "id = ?", receiptID).Error
		if err == nil {
			if old.Digest != hex.EncodeToString(digest[:]) {
				return failure(req.ID, 409, "请求编号已用于其他操作")
			}
			if old.Status == 0 {
				return failure(req.ID, 409, "操作结果待核实，请刷新任务，不要重复提交")
			}
			return Response{ID: req.ID, Status: old.Status, Body: old.Body}
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return failure(req.ID, 500, "无法读取请求记录")
		}
		if s.db.Create(&receipt{ID: receiptID, Digest: hex.EncodeToString(digest[:]), Deadline: req.Deadline}).Error != nil {
			return failure(req.ID, 500, "无法保存请求记录")
		}
	}
	// Only the fixed allowlist reaches the local router. No client-supplied host,
	// authorization, origin, or arbitrary headers are forwarded.
	local := httptest.NewRequest(req.Method, "http://127.0.0.1"+req.Path, bytes.NewReader(req.Body))
	local.RemoteAddr = "127.0.0.1:1"
	local.Header.Set("Content-Type", "application/json")
	local.Header.Set("X-Bot-ID", req.BotID)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, local.WithContext(ctx))
	body := rec.Body.Bytes()
	if len(body) == 0 {
		body = []byte(`null`)
	}
	if len(body) > 1<<20 || !json.Valid(body) {
		body = []byte(`{"error":"结果过大，请在 Mac 查看"}`)
		rec.Code = 413
	}
	res := Response{ID: req.ID, Status: rec.Code, Body: append([]byte{}, body...)}
	if write {
		if s.db.Model(&receipt{}).Where("id = ?", receiptID).Updates(map[string]any{"status": res.Status, "body": []byte(res.Body)}).Error != nil {
			return failure(req.ID, 409, "操作结果待核实，请刷新任务")
		}
	}
	return res
}
func allowed(method, path string) bool {
	if method == "GET" {
		switch path {
		case "/api/health", "/api/bots", "/api/runs", "/api/memories", "/api/conversations":
			return true
		}
		return runPath.MatchString(path) && (strings.HasSuffix(path, "/tools") || strings.HasSuffix(path, "/artifacts"))
	}
	return method == "POST" && (path == "/api/runs" || decisionPath.MatchString(path) || runPath.MatchString(path) && (strings.HasSuffix(path, "/cancel") || strings.HasSuffix(path, "/resume")))
}
func (s *Service) Register(r *gin.Engine) {
	r.GET("/api/mobile", func(c *gin.Context) {
		s.mu.Lock()
		defer s.mu.Unlock()
		devices := []Device{}
		for _, d := range s.state.Devices {
			devices = append(devices, d)
		}
		var pending any
		if s.pair != nil && s.pair.Expires >= time.Now().Unix() && s.pair.Pending != nil {
			pending = gin.H{"device": s.pair.Pending, "code": s.pair.Code, "expiresAt": s.pair.Expires}
		}
		c.JSON(200, gin.H{"available": s.relayURL != "", "enabled": s.state.Enabled, "online": s.online, "devices": devices, "pending": pending})
	})
	r.POST("/api/mobile/enabled", func(c *gin.Context) {
		var input struct {
			Enabled bool `json:"enabled"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.Status(400)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if input.Enabled && s.relayURL == "" {
			c.JSON(409, gin.H{"error": "远程连接服务尚未部署"})
			return
		}
		old := s.state.Enabled
		s.state.Enabled = input.Enabled
		if e := s.save(); e != nil {
			s.state.Enabled = old
			c.JSON(500, gin.H{"error": "保存失败"})
			return
		}
		if !input.Enabled {
			s.pair = nil
			s.online = false
		}
		c.Status(204)
	})
	r.POST("/api/mobile/pairing", func(c *gin.Context) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !s.state.Enabled || !s.online {
			c.JSON(409, gin.H{"error": "请先开启连接并等待中继就绪"})
			return
		}
		s.pair = &pairing{Secret: relay.RandomID(), Expires: time.Now().Add(5 * time.Minute).Unix()}
		payload := gin.H{"v": 1, "relay": s.relayURL, "host": relay.HostID(s.state.HostToken), "publicKey": b64(s.state.Public[:]), "secret": s.pair.Secret, "expiresAt": s.pair.Expires}
		raw, _ := json.Marshal(payload)
		c.JSON(200, gin.H{"url": s.mobileURL + "/mobile.html#pair=" + b64(raw), "expiresAt": s.pair.Expires})
	})
	r.POST("/api/mobile/decision", func(c *gin.Context) {
		var input struct {
			Key     string `json:"key"`
			Code    string `json:"code"`
			Approve bool   `json:"approve"`
		}
		if c.ShouldBindJSON(&input) != nil {
			c.Status(400)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		p := s.pair
		if p == nil || p.Expires < time.Now().Unix() || p.Pending == nil || p.Pending.Key != input.Key || p.Code != input.Code {
			c.JSON(409, gin.H{"error": "配对请求已变化或过期"})
			return
		}
		if input.Approve {
			if len(s.state.Devices) >= 10 {
				c.JSON(409, gin.H{"error": "最多关联 10 台手机"})
				return
			}
			s.state.Devices[input.Key] = *p.Pending
			if s.save() != nil {
				delete(s.state.Devices, input.Key)
				c.JSON(500, gin.H{"error": "保存设备失败"})
				return
			}
		}
		s.pair = nil
		c.Status(204)
	})
	r.DELETE("/api/mobile/devices/:key", func(c *gin.Context) {
		s.mu.Lock()
		defer s.mu.Unlock()
		key := c.Param("key")
		old, ok := s.state.Devices[key]
		delete(s.state.Devices, key)
		if s.save() != nil {
			if ok {
				s.state.Devices[key] = old
			}
			c.JSON(500, gin.H{"error": "撤销失败"})
			return
		}
		c.Status(204)
	})
}
func (s *Service) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		s.mu.Lock()
		enabled := s.state.Enabled
		configured := s.relayURL != ""
		s.mu.Unlock()
		if !enabled || !configured {
			if !pause(ctx, time.Second) {
				return
			}
			continue
		}
		res, err := s.transport(ctx, "GET", "poll", nil)
		if err != nil {
			s.setOnline(false)
			if !pause(ctx, backoff) {
				return
			}
			if backoff < 15*time.Second {
				backoff *= 2
			}
			continue
		}
		s.setOnline(true)
		backoff = time.Second
		if res.StatusCode == 200 {
			var packet relay.Packet
			err = json.NewDecoder(io.LimitReader(res.Body, relay.MaxEnvelope)).Decode(&packet)
			res.Body.Close()
			if err == nil {
				packet.Envelope = s.Handle(packet.Envelope)
				raw, _ := json.Marshal(packet)
				reply, e := s.transport(ctx, "POST", "reply", raw)
				if e == nil {
					reply.Body.Close()
				}
			}
		} else {
			res.Body.Close()
			if res.StatusCode != 204 {
				s.setOnline(false)
				pause(ctx, 5*time.Second)
			}
		}
		s.db.Where("deadline < ?", time.Now().Unix()-300).Delete(&receipt{})
	}
}
func (s *Service) setOnline(value bool) {
	s.mu.Lock()
	s.online = value && s.state.Enabled
	s.mu.Unlock()
}
func (s *Service) transport(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	r, e := http.NewRequestWithContext(ctx, method, s.relayURL+"/v1/hosts/"+relay.HostID(s.state.HostToken)+"/"+path, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	r.Header.Set("Authorization", "Bearer "+s.state.HostToken)
	r.Header.Set("Content-Type", "application/json")
	return s.client.Do(r)
}
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
