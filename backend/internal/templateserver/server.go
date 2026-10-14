// Package templateserver serves portable configurations independently of the
// desktop's private API, memory database and model credentials.
package templateserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/bottemplate"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

type Publisher struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TokenHash string `json:"tokenHash"`
}
type Template struct {
	ID       string `gorm:"primaryKey"`
	Owner    string `gorm:"index"`
	Current  int
	Disabled bool
	Deleted  bool
}
type Revision struct {
	ID          uint   `gorm:"primaryKey"`
	TemplateID  string `gorm:"uniqueIndex:template_revision"`
	Number      int    `gorm:"uniqueIndex:template_revision"`
	Owner       string `gorm:"uniqueIndex:publisher_request"`
	RequestID   string `gorm:"uniqueIndex:publisher_request"`
	RequestHash string
	Content     string `gorm:"type:text"`
	Digest      string
	Publisher   string
	CreatedAt   time.Time
}
type Server struct {
	db         *gorm.DB
	origin     string
	publishers map[string]Publisher
	mu         sync.Mutex
	limit      *rate.Limiter
}

func New(db *gorm.DB, origin string, publishers []Publisher) (*Server, error) {
	base, e := bottemplate.ServiceURL(origin)
	if e != nil {
		return nil, e
	}
	s := &Server{db: db, origin: base, publishers: map[string]Publisher{}, limit: rate.NewLimiter(50, 100)}
	owners := map[string]bool{}
	for _, p := range publishers {
		hash, e := hex.DecodeString(p.TokenHash)
		if e != nil || len(hash) != sha256.Size || p.ID == "" || len(p.ID) > 100 || p.Name == "" || len([]rune(p.Name)) > 100 || owners[p.ID] {
			return nil, errors.New("发布者配置无效或 ID 重复")
		}
		key := strings.ToLower(p.TokenHash)
		if _, ok := s.publishers[key]; ok {
			return nil, errors.New("发布凭据不能重复")
		}
		s.publishers[key] = p
		owners[p.ID] = true
	}
	if len(s.publishers) == 0 {
		return nil, errors.New("至少配置一位发布者；请先生成发布凭据")
	}
	if e = db.AutoMigrate(&Template{}, &Revision{}); e != nil {
		return nil, e
	}
	return s, nil
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /api/me", s.auth(func(w http.ResponseWriter, r *http.Request, p Publisher) {
		respond(w, 200, map[string]string{"name": p.Name})
	}))
	m.HandleFunc("POST /api/templates", s.auth(s.publish))
	m.HandleFunc("POST /api/templates/{id}/versions", s.auth(s.publish))
	m.HandleFunc("GET /api/templates", s.auth(s.list))
	m.HandleFunc("PATCH /api/templates/{id}", s.auth(s.manage))
	m.HandleFunc("DELETE /api/templates/{id}", s.auth(s.manage))
	m.HandleFunc("GET /api/templates/{id}/versions/{revision}", s.fetch)
	m.HandleFunc("GET /t/{id}/v/{revision}", s.preview)
	m.HandleFunc("GET /assets/import.js", serveImportScript)
	m.HandleFunc("GET /assets/avatars/{name}", serveAvatar)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		if !s.limit.Allow() {
			fail(w, 429, "请求较多，请稍后重试")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, bottemplate.MaxBytes)
		m.ServeHTTP(w, r)
	})
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func read(r *http.Request, v any) error {
	data, e := io.ReadAll(r.Body)
	if e != nil {
		return errors.New("请求内容超过限制")
	}
	return bottemplate.Decode(data, v)
}
func (s *Server) auth(next func(http.ResponseWriter, *http.Request, Publisher)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || len(token) < 32 || len(token) > 256 {
			fail(w, 401, "请配置有效的发布凭据")
			return
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
		p, ok := s.publishers[hash]
		if !ok {
			fail(w, 401, "发布凭据无效或已撤销")
			return
		}
		next(w, r, p)
	}
}
func (s *Server) published(t Template, v Revision) bottemplate.Published {
	var config bottemplate.Config
	_ = json.Unmarshal([]byte(v.Content), &config)
	return bottemplate.Published{ID: t.ID, Revision: v.Number, Digest: v.Digest, Publisher: v.Publisher, CreatedAt: v.CreatedAt, Disabled: t.Disabled, Config: config, URL: fmt.Sprintf("%s/t/%s/v/%d", s.origin, t.ID, v.Number)}
}
func (s *Server) publish(w http.ResponseWriter, r *http.Request, p Publisher) {
	var input bottemplate.PublishRequest
	if e := read(r, &input); e != nil {
		fail(w, 400, e.Error())
		return
	}
	if !bottemplate.ValidID(input.RequestID) {
		fail(w, 400, "请求标识无效")
		return
	}
	if e := input.Config.Validate(); e != nil {
		fail(w, 400, e.Error())
		return
	}
	id := r.PathValue("id")
	if id != "" && !bottemplate.ValidID(id) {
		fail(w, 400, "模板标识无效")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	digest := bottemplate.Digest(input.Config)
	requestHash := id + ":" + digest
	var t Template
	var v Revision
	code := 201
	e := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("owner = ? AND request_id = ?", p.ID, input.RequestID).First(&v).Error
		if err == nil {
			if v.RequestHash != requestHash {
				return errors.New("请求标识已用于其他内容")
			}
			code = 200
			return tx.Where("id = ? AND deleted = ?", v.TemplateID, false).First(&t).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if id == "" {
			t = Template{ID: uuid.NewString(), Owner: p.ID}
			if err = tx.Create(&t).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Where("id = ? AND owner = ? AND deleted = ?", id, p.ID, false).First(&t).Error; err != nil {
				return err
			}
		}
		if t.Disabled {
			return errors.New("请先启用模板，再发布新版")
		}
		t.Current++
		raw, _ := json.Marshal(input.Config)
		v = Revision{TemplateID: t.ID, Number: t.Current, Owner: p.ID, RequestID: input.RequestID, RequestHash: requestHash, Content: string(raw), Digest: digest, Publisher: p.Name}
		if err = tx.Create(&v).Error; err != nil {
			return err
		}
		return tx.Save(&t).Error
	})
	if e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			fail(w, 404, "模板不存在或无权管理")
		} else {
			fail(w, 409, "发布失败，请检查请求标识或模板状态")
		}
		return
	}
	respond(w, code, s.published(t, v))
}
func (s *Server) list(w http.ResponseWriter, r *http.Request, p Publisher) {
	var rows []Template
	query := s.db.Where("owner = ? AND deleted = ?", p.ID, false)
	if after := r.URL.Query().Get("after"); after != "" {
		if !bottemplate.ValidID(after) {
			fail(w, 400, "分页标识无效")
			return
		}
		query = query.Where("id > ?", after)
	}
	if query.Order("id").Limit(50).Find(&rows).Error != nil {
		fail(w, 500, "无法加载模板")
		return
	}
	out := []bottemplate.Published{}
	for _, t := range rows {
		var v Revision
		if s.db.Where("template_id = ? AND number = ?", t.ID, t.Current).First(&v).Error != nil {
			fail(w, 500, "无法加载版本")
			return
		}
		item := s.published(t, v)
		// Listing needs metadata only; large role definitions are fetched separately.
		item.Config = bottemplate.Config{Format: bottemplate.Format, SchemaVersion: 1, Name: item.Config.Name, Avatar: item.Config.Avatar}
		out = append(out, item)
	}
	respond(w, 200, out)
}
func (s *Server) manage(w http.ResponseWriter, r *http.Request, p Publisher) {
	var input struct {
		Disabled *bool `json:"disabled"`
	}
	if r.Method == http.MethodPatch {
		if e := read(r, &input); e != nil || input.Disabled == nil {
			fail(w, 400, "请指定模板状态")
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.db.Transaction(func(tx *gorm.DB) error {
		var t Template
		if e := tx.Where("id = ? AND owner = ? AND deleted = ?", r.PathValue("id"), p.ID, false).First(&t).Error; e != nil {
			return e
		}
		if r.Method == http.MethodDelete {
			t.Disabled = true
			t.Deleted = true
			// Keep request receipts to prevent a late retry from republishing deleted content.
			if e := tx.Model(&Revision{}).Where("template_id = ?", t.ID).Update("content", "").Error; e != nil {
				return e
			}
		} else {
			t.Disabled = *input.Disabled
		}
		return tx.Save(&t).Error
	})
	if e != nil {
		fail(w, 404, "模板不存在或无权管理")
		return
	}
	w.WriteHeader(204)
}
func (s *Server) load(r *http.Request) (bottemplate.Published, error) {
	id := r.PathValue("id")
	n, e := strconv.Atoi(r.PathValue("revision"))
	if e != nil || n < 1 || !bottemplate.ValidID(id) {
		return bottemplate.Published{}, gorm.ErrRecordNotFound
	}
	var t Template
	var v Revision
	e = s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Where("id = ? AND disabled = ? AND deleted = ?", id, false, false).First(&t).Error; e != nil {
			return e
		}
		return tx.Where("template_id = ? AND number = ?", id, n).First(&v).Error
	})
	return s.published(t, v), e
}
func (s *Server) fetch(w http.ResponseWriter, r *http.Request) {
	v, e := s.load(r)
	if e != nil {
		fail(w, 404, "模板不存在、已停用或已删除")
		return
	}
	respond(w, 200, v)
}
