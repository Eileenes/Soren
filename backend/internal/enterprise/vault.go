package enterprise

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"

	"github.com/soren/backend/internal/connectors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type credential struct {
	ID         string `gorm:"primaryKey"`
	Ciphertext []byte
}
type vault struct {
	db        *gorm.DB
	aead      cipher.AEAD
	workspace string
}

func newVault(db *gorm.DB, key []byte, workspace string) (*vault, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err = db.AutoMigrate(&credential{}); err != nil {
		return nil, err
	}
	return &vault{db: db, aead: aead, workspace: workspace}, nil
}
func (v *vault) Get(id string) (connectors.Credential, error) {
	var row credential
	var c connectors.Credential
	if err := v.db.First(&row, "id = ?", id).Error; err != nil {
		return c, errors.New("凭据不存在")
	}
	n := v.aead.NonceSize()
	if len(row.Ciphertext) < n {
		return c, errors.New("凭据损坏")
	}
	raw, err := v.aead.Open(nil, row.Ciphertext[:n], row.Ciphertext[n:], []byte(v.workspace+":"+id))
	if err != nil {
		return c, errors.New("无法解密凭据，请检查服务端密钥")
	}
	err = json.Unmarshal(raw, &c)
	return c, err
}
func (v *vault) Set(id string, c connectors.Credential) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	nonce := make([]byte, v.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	sealed := v.aead.Seal(nonce, nonce, raw, []byte(v.workspace+":"+id))
	return v.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"ciphertext"})}).Create(&credential{ID: id, Ciphertext: sealed}).Error
}
func (v *vault) Delete(id string) error { return v.db.Delete(&credential{}, "id = ?", id).Error }
