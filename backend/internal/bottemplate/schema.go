package bottemplate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/botconfig"
)

const MaxBytes = 256 << 10
const Format = "soren.bot-template"

type Config struct {
	Format               string               `json:"format"`
	SchemaVersion        int                  `json:"schemaVersion"`
	Name                 string               `json:"name"`
	Description          string               `json:"description"`
	Avatar               string               `json:"avatar"`
	Instructions         string               `json:"instructions"`
	Definition           botconfig.Definition `json:"definition"`
	PolicyTemplate       *botconfig.Policy    `json:"policyTemplate"`
	MemoryEnabled        bool                 `json:"memoryEnabled"`
	RequiredCapabilities []string             `json:"requiredCapabilities"`
}
type Published struct {
	ID        string    `json:"id"`
	Revision  int       `json:"revision"`
	Digest    string    `json:"digest"`
	Publisher string    `json:"publisher"`
	CreatedAt time.Time `json:"createdAt"`
	URL       string    `json:"url"`
	Disabled  bool      `json:"disabled"`
	Config    Config    `json:"config"`
}
type PublishRequest struct {
	RequestID string `json:"requestId"`
	Config    Config `json:"config"`
}

func Decode(data []byte, target any) error {
	if len(data) > MaxBytes {
		return errors.New("模板超过 256 KB 限制")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return errors.New("模板格式无效或包含不支持的字段")
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("模板包含多余内容")
	}
	return nil
}
func (c Config) Validate() error {
	if c.Format != Format || c.SchemaVersion != 1 {
		return errors.New("不支持此模板格式，请更新 Soren")
	}
	if strings.TrimSpace(c.Name) == "" || len([]rune(c.Name)) > 40 || len([]rune(c.Description)) > 1000 || strings.TrimSpace(c.Instructions) == "" || len([]rune(c.Instructions)) > 8000 {
		return errors.New("请填写名称和职责，名称最多 40 字，简介最多 1000 字，职责最多 8000 字")
	}
	if len(c.Avatar) > 40 || len(c.RequiredCapabilities) > 128 {
		return errors.New("模板字段超出限制")
	}
	for _, key := range c.RequiredCapabilities {
		if !botconfig.ActionPattern.MatchString(key) {
			return errors.New("能力标识无效")
		}
	}
	if err := botconfig.Validate(c.Definition, c.PolicyTemplate); err != nil {
		return err
	}
	raw, _ := json.Marshal(c)
	if len(raw) > MaxBytes-4096 {
		return errors.New("模板内容过长")
	}
	return nil
}
func Digest(c Config) string { raw, _ := json.Marshal(c); return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func ValidID(id string) bool { parsed, e := uuid.Parse(id); return e == nil && parsed.String() == id }
func ServiceURL(raw string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil {
		return "", errors.New("服务地址无效")
	}
	loopback := u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()
	if u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopback)) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("请填写 HTTPS 服务根地址；本机调试可用 HTTP 回环地址")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}
