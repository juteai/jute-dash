package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"
	"gorm.io/gorm"
)

const (
	MasterKeySize = 32
)

var ErrSecretNotFound = errors.New("secret not found")

type SecretDB struct {
	ID         string `gorm:"primaryKey;column:id"`
	Kind       string `gorm:"column:kind"`
	Ciphertext []byte `gorm:"column:ciphertext"`
	Nonce      []byte `gorm:"column:nonce"`
	CreatedAt  string `gorm:"column:created_at"`
	UpdatedAt  string `gorm:"column:updated_at"`
}

func (SecretDB) TableName() string {
	return "secrets"
}

type MasterKeyProvider interface {
	MasterKey(ctx context.Context) ([]byte, error)
}

type Vault struct {
	db       *gorm.DB
	provider MasterKeyProvider
}

func NewVault(db *gorm.DB, provider MasterKeyProvider) *Vault {
	return &Vault{db: db, provider: provider}
}

func (v *Vault) Store(ctx context.Context, id string, kind string, value string) error {
	id = strings.TrimSpace(id)
	kind = strings.TrimSpace(kind)
	if id == "" {
		return errors.New("secret id is required")
	}
	if kind == "" {
		return errors.New("secret kind is required")
	}
	key, err := v.masterKey(ctx)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create secret AEAD: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate secret nonce: %w", err)
	}
	now := nowUTC()
	row := SecretDB{
		ID:         id,
		Kind:       kind,
		Ciphertext: gcm.Seal(nil, nonce, []byte(value), []byte(id)),
		Nonce:      nonce,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	err = v.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing SecretDB
		if err := tx.First(&existing, "id = ?", id).Error; err == nil {
			row.CreatedAt = existing.CreatedAt
		}
		return tx.Save(&row).Error
	})
	if err != nil {
		return fmt.Errorf("store secret: %w", err)
	}
	return nil
}

func (v *Vault) Resolve(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrSecretNotFound
	}
	var row SecretDB
	if err := v.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrSecretNotFound
		}
		return "", fmt.Errorf("load secret: %w", err)
	}
	key, err := v.masterKey(ctx)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create secret cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create secret AEAD: %w", err)
	}
	plaintext, err := gcm.Open(nil, row.Nonce, row.Ciphertext, []byte(id))
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func (v *Vault) masterKey(ctx context.Context) ([]byte, error) {
	if v == nil || v.provider == nil {
		return nil, errors.New("secret vault is unavailable")
	}
	key, err := v.provider.MasterKey(ctx)
	if err != nil {
		return nil, err
	}
	if len(key) != MasterKeySize {
		return nil, fmt.Errorf("secret master key must be %d bytes", MasterKeySize)
	}
	return key, nil
}

type EnvMasterKeyProvider struct {
	Value string
}

func (p EnvMasterKeyProvider) MasterKey(context.Context) ([]byte, error) {
	return DecodeMasterKey(p.Value)
}

type KeyringMasterKeyProvider struct {
	EnvName string
	KeyID   string

	mu     sync.Mutex
	cached []byte
}

func NewKeyringMasterKeyProvider() *KeyringMasterKeyProvider {
	return &KeyringMasterKeyProvider{
		EnvName: "JUTE_SECRET_KEY",
		KeyID:   "jute-dash/master-key/v1",
	}
}

func (p *KeyringMasterKeyProvider) MasterKey(context.Context) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.cached) == MasterKeySize {
		return append([]byte(nil), p.cached...), nil
	}
	if p.EnvName != "" {
		if env := strings.TrimSpace(getenv(p.EnvName)); env != "" {
			key, err := DecodeMasterKey(env)
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", p.EnvName, err)
			}
			p.cached = key
			return append([]byte(nil), key...), nil
		}
	}
	ring, err := keyring.Open(keyring.Config{
		ServiceName: "Jute Dash",
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.WinCredBackend,
			keyring.SecretServiceBackend,
			keyring.KWalletBackend,
			keyring.PassBackend,
			keyring.KeyCtlBackend,
		},
		KeychainTrustApplication:       true,
		KeychainAccessibleWhenUnlocked: true,
		WinCredPrefix:                  "Jute Dash",
		KWalletAppID:                   "jute-dash",
		KWalletFolder:                  "Jute Dash",
		LibSecretCollectionName:        "default",
		PassPrefix:                     "jute-dash",
		KeyCtlScope:                    "user",
		KeychainSynchronizable:         false,
	})
	if err != nil {
		return nil, fmt.Errorf("open OS credential store: %w", err)
	}
	keyID := p.KeyID
	if keyID == "" {
		keyID = "jute-dash/master-key/v1"
	}
	item, err := ring.Get(keyID)
	if err == nil {
		key, err := DecodeMasterKey(string(item.Data))
		if err != nil {
			return nil, fmt.Errorf("decode stored master key: %w", err)
		}
		p.cached = key
		return append([]byte(nil), key...), nil
	}
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		return nil, fmt.Errorf("load master key from OS credential store: %w", err)
	}
	key := make([]byte, MasterKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := ring.Set(keyring.Item{
		Key:         keyID,
		Data:        []byte(encoded),
		Label:       "Jute Dash secret vault master key",
		Description: "Encrypts local Jute Dash secrets stored in SQLite.",
	}); err != nil {
		return nil, fmt.Errorf("store master key in OS credential store: %w", err)
	}
	p.cached = key
	return append([]byte(nil), key...), nil
}

func DecodeMasterKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("master key is empty")
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == MasterKeySize {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == MasterKeySize {
		return decoded, nil
	}
	if len(value) == MasterKeySize {
		return []byte(value), nil
	}
	return nil, fmt.Errorf("master key must be %d raw bytes, hex bytes, or base64 bytes", MasterKeySize)
}

//nolint:gochecknoglobals // test seam for environment-backed master key.
var getenv = os.Getenv

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
