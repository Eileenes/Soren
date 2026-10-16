package enterprise

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type User struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Provider  string    `gorm:"uniqueIndex:enterprise_identity" json:"-"`
	Subject   string    `gorm:"uniqueIndex:enterprise_identity" json:"-"`
	Name      string    `json:"name"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"createdAt"`
}

func (User) TableName() string { return "enterprise_users" }

type session struct {
	Hash      string    `gorm:"primaryKey"`
	UserID    string    `gorm:"index"`
	ExpiresAt time.Time `gorm:"index"`
}

func (session) TableName() string { return "enterprise_sessions" }

type WorkspaceFactory func(User) (http.Handler, error)
type Server struct {
	Config     Config
	DB         *gorm.DB
	HTTP       *http.Client
	Factory    WorkspaceFactory
	mu         sync.Mutex
	attempts   map[string]*attempt
	loginLimit *rate.Limiter
	profiles   profileVerifier
}

func New(c Config, db *gorm.DB, factory WorkspaceFactory) (*Server, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&User{}, &session{}); err != nil {
		return nil, err
	}
	return &Server{Config: c, DB: db, Factory: factory, attempts: map[string]*attempt{}, loginLimit: rate.NewLimiter(5, 20),
		HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func secret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}

var (
	errUnauthorized = errors.New("登录已失效或账号已停用，请重新登录")
	errUnavailable  = errors.New("公司服务暂不可用，请稍后重试")
)

func (s *Server) activeUser(db *gorm.DB, id string) (User, error) {
	var u User
	if err := db.First(&u, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return u, errUnauthorized
		}
		return u, errUnavailable
	}
	if u.Disabled || (!s.Config.AllowedSubjects["*"] && !s.Config.AllowedSubjects[u.Subject]) || u.Provider != s.Config.identityProvider() {
		return u, errUnauthorized
	}
	return u, nil
}
func (s *Server) Active(id string) error {
	_, err := s.activeUser(s.DB, id)
	return err
}
func (s *Server) authenticate(r *http.Request) (User, error) {
	var u User
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(token) != 43 {
		return u, errUnauthorized
	}
	db := s.DB.WithContext(r.Context())
	var row session
	if err := db.First(&row, "hash = ? AND expires_at > ?", hash(token), time.Now()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return u, errUnauthorized
		}
		return u, errUnavailable
	}
	return s.activeUser(db, row.UserID)
}
func (s *Server) identity(u User) map[string]any {
	return map[string]any{"user": u, "workspaceId": u.ID, "organization": s.Config.Name}
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		sql, err := s.DB.DB()
		if err != nil || sql.PingContext(r.Context()) != nil {
			fail(w, 503, "数据库不可用")
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /.well-known/soren", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"product": "soren", "protocolVersion": 1, "name": s.Config.Name, "serverUrl": s.Config.PublicURL, "auth": "oauth-browser", "capabilities": []string{"personal-workspace", "chat", "bots", "tasks", "connections"}})
	})
	m.HandleFunc("POST /auth/attempts", s.startLogin)
	m.HandleFunc("GET /auth/browser", s.browserLogin)
	if s.Config.AuthMode == "ingresshub" {
		m.HandleFunc("POST /auth/confirm", s.confirmIngressHub)
	} else {
		m.HandleFunc("GET /auth/callback", s.callback)
	}
	m.HandleFunc("POST /auth/poll", s.pollLogin)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		u, err := s.authenticate(r)
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, errUnauthorized) {
				status = http.StatusUnauthorized
			}
			fail(w, status, err.Error())
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/session" {
			respond(w, 200, s.identity(u))
			return
		}
		if r.Method == "POST" && r.URL.Path == "/api/session/logout" {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if s.DB.Delete(&session{}, "hash = ?", hash(token)).Error != nil {
				fail(w, 503, "退出失败，请重试")
				return
			}
			w.WriteHeader(204)
			return
		}
		h, err := s.Factory(u)
		if err != nil {
			fail(w, 503, "个人空间暂不可用，请稍后重试")
			return
		}
		// Only the authenticated subject selects a workspace. Ignore all client owner hints.
		r.Header.Del("X-User-ID")
		r.Header.Del("X-Workspace-ID")
		r.Header.Del("Authorization")
		h.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		// Native clients use their local authenticated bridge; no cross-origin browser API access.
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.Config.PublicURL {
			fail(w, 403, "来源不受信任")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		m.ServeHTTP(w, r)
	})
}
func (s *Server) saveUser(subject, name string) (User, error) {
	// 查询时不能预填随机主键：GORM 会将主键加入查询条件，旧账号永远查不到。
	var u User
	provider := s.Config.identityProvider()
	err := s.DB.Where("provider = ? AND subject = ?", provider, subject).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		candidate := User{ID: uuid.NewString(), Provider: provider, Subject: subject, Name: name}
		// 多个登录请求同时首次建号时，只保留一个身份，不覆盖停用状态或用户 ID。
		err = s.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "provider"}, {Name: "subject"}}, DoNothing: true,
		}).Create(&candidate).Error
		if err == nil {
			err = s.DB.Where("provider = ? AND subject = ?", provider, subject).First(&u).Error
		}
	}
	if err != nil {
		return u, err
	}
	if err = s.Active(u.ID); err != nil {
		return u, err
	}
	return u, s.DB.Model(&u).Update("name", name).Error
}
