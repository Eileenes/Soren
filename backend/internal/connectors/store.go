package connectors

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type Connection struct {
	ID          string       `gorm:"primaryKey" json:"id"`
	Kind        string       `json:"kind"`
	Name        string       `json:"name"`
	Account     string       `json:"account"`
	Endpoint    string       `json:"endpoint,omitempty"`
	AllowedBots []string     `gorm:"serializer:json;type:text" json:"allowedBots"`
	Tools       []RemoteTool `gorm:"serializer:json;type:text" json:"tools"`
	Enabled     bool         `json:"enabled"`
	Version     int          `json:"version"`
	CreatedAt   time.Time    `json:"createdAt"`
}
type RemoteTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}
type Credential struct {
	Token        *oauth2.Token `json:"token,omitempty"`
	APIKey       string        `json:"apiKey,omitempty"`
	ClientID     string        `json:"clientId,omitempty"`
	ClientSecret string        `json:"clientSecret,omitempty"`
}
type Vault interface {
	Get(string) (Credential, error)
	Set(string, Credential) error
	Delete(string) error
}
type Keychain struct{ Namespace string }

func NewKeychain(dir string) *Keychain {
	return &Keychain{Namespace: fmt.Sprintf("app.soren.connections.%x", sha256.Sum256([]byte(dir)))}
}
func (v *Keychain) Get(id string) (Credential, error) {
	var c Credential
	raw, e := keyring.Get(v.Namespace, id)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal([]byte(raw), &c)
	return c, e
}
func (v *Keychain) Set(id string, c Credential) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	return keyring.Set(v.Namespace, id, string(b))
}
func (v *Keychain) Delete(id string) error {
	e := keyring.Delete(v.Namespace, id)
	if e == keyring.ErrNotFound {
		return nil
	}
	return e
}

// MemoryVault is injected by tests; production never silently falls back to it.
type MemoryVault struct {
	mu   sync.Mutex
	Data map[string]Credential
}

func (v *MemoryVault) Get(id string) (Credential, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.Data[id]
	if !ok {
		return c, fmt.Errorf("凭据不存在")
	}
	return c, nil
}
func (v *MemoryVault) Set(id string, c Credential) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.Data == nil {
		v.Data = map[string]Credential{}
	}
	v.Data[id] = c
	return nil
}
func (v *MemoryVault) Delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.Data, id)
	return nil
}

type Invocation struct {
	PolicyRevision    string    `json:"-"`
	ID                string    `gorm:"primaryKey" json:"id"`
	RunID             string    `gorm:"index" json:"runId"`
	BotID             string    `json:"botId"`
	ConnectionID      string    `json:"connectionId"`
	ConnectionName    string    `json:"connectionName"`
	Account           string    `json:"account"`
	ConnectionVersion int       `json:"-"`
	Tool              string    `json:"tool"`
	Arguments         string    `gorm:"type:text" json:"arguments"`
	Status            string    `json:"status"`
	Result            string    `gorm:"type:text" json:"result"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
type Transcript struct {
	ID       string `gorm:"primaryKey"`
	Messages string `gorm:"type:text"`
	Rounds   int
}
type Identity struct {
	ID       uint      `gorm:"primaryKey"`
	GitHubID int64     `json:"githubId"`
	Login    string    `json:"login"`
	Name     string    `json:"name"`
	BoundAt  time.Time `json:"boundAt"`
}

func migrate(db *gorm.DB) error {
	if e := db.AutoMigrate(&Connection{}, &Invocation{}, &Transcript{}, &Identity{}, &Artifact{}); e != nil {
		return e
	}
	return db.Model(&Invocation{}).Where("status = ?", "running").Updates(map[string]any{"status": "unknown", "result": "服务在操作期间退出，结果待核实；不会自动重试。"}).Error
}
