// Package enterprise provides the authenticated, single-organization server.
// It is deliberately separate from the desktop's unauthenticated loopback API.
package enterprise

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PublicURL, Name, Listen, DatabaseURL, DataDir                   string
	AuthorizationURL, TokenURL, UserInfoURL, ClientID, ClientSecret string
	SubjectClaim, Scope, AuthMethod                                 string
	AllowedSubjects                                                 map[string]bool
	Key                                                             []byte
	Development                                                     bool
	SessionTTL                                                      time.Duration
	Concurrency                                                     int
}

func Origin(raw string, development bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("请填写不包含路径、账号或参数的 HTTPS 服务器地址")
	}
	local := u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()
	if u.Scheme != "https" && !(development && local && u.Scheme == "http") {
		return "", errors.New("服务器须使用 HTTPS；开发模式仅允许回环 HTTP 地址")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

func LoadConfig() (Config, error) {
	value := func(k, fallback string) string {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
		return fallback
	}
	c := Config{
		PublicURL: value("ENTERPRISE_PUBLIC_URL", ""), Name: value("ENTERPRISE_NAME", "公司空间"),
		Listen: value("ENTERPRISE_LISTEN", "127.0.0.1:8091"), DatabaseURL: value("DATABASE_URL", ""),
		DataDir: value("SOREN_DATA_DIR", "./data"), Development: os.Getenv("ENTERPRISE_DEV") == "1",
		AuthorizationURL: value("ENTERPRISE_AUTHORIZATION_URL", ""), TokenURL: value("ENTERPRISE_TOKEN_URL", ""),
		UserInfoURL: value("ENTERPRISE_USERINFO_URL", ""), ClientID: value("ENTERPRISE_CLIENT_ID", ""),
		ClientSecret: os.Getenv("ENTERPRISE_CLIENT_SECRET"), SubjectClaim: value("ENTERPRISE_SUBJECT_CLAIM", "sub"),
		Scope: value("ENTERPRISE_SCOPE", "openid profile email"), AuthMethod: value("ENTERPRISE_AUTH_METHOD", "client_secret_basic"),
		SessionTTL: 8 * time.Hour, Concurrency: 8, AllowedSubjects: map[string]bool{},
	}
	for _, id := range strings.Split(value("ENTERPRISE_ALLOWED_SUBJECTS", ""), ",") {
		if id = strings.TrimSpace(id); id != "" {
			c.AllowedSubjects[id] = true
		}
	}
	c.Key, _ = base64.StdEncoding.DecodeString(os.Getenv("ENTERPRISE_VAULT_KEY"))
	if n := os.Getenv("ENTERPRISE_MODEL_CONCURRENCY"); n != "" {
		c.Concurrency, _ = strconv.Atoi(n)
	}
	err := c.Validate()
	return c, err
}

func (c *Config) Validate() error {
	var err error
	if c.PublicURL, err = Origin(c.PublicURL, c.Development); err != nil {
		return err
	}
	if len(c.Key) != 32 {
		return errors.New("ENTERPRISE_VAULT_KEY 必须是 Base64 编码的 32 字节随机密钥")
	}
	if c.DatabaseURL == "" || c.ClientID == "" || c.ClientSecret == "" || len(c.AllowedSubjects) == 0 {
		return errors.New("请配置 DATABASE_URL、OAuth 客户端凭据和 ENTERPRISE_ALLOWED_SUBJECTS")
	}
	db, e := url.Parse(c.DatabaseURL)
	if e != nil || (db.Scheme != "postgres" && db.Scheme != "postgresql") || db.Hostname() == "" {
		return errors.New("DATABASE_URL 必须是 PostgreSQL URL")
	}
	if c.AuthMethod != "client_secret_basic" && c.AuthMethod != "client_secret_post" {
		return errors.New("不支持的 OAuth 客户端认证方式")
	}
	for _, raw := range []string{c.AuthorizationURL, c.TokenURL, c.UserInfoURL} {
		u, e := url.Parse(raw)
		if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("OAuth 接口地址无效")
		}
		if _, e = Origin(u.Scheme+"://"+u.Host, c.Development); e != nil {
			return e
		}
	}
	if c.Concurrency < 1 || c.Concurrency > 128 || c.SessionTTL <= 0 || c.SessionTTL > 24*time.Hour || c.SubjectClaim == "" || len(c.Name) > 200 {
		return errors.New("企业服务参数无效")
	}
	return nil
}
