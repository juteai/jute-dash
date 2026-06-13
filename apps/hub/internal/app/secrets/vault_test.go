package secrets

import (
	"context"
	"encoding/base64"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestVaultStoresEncryptedSecrets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&SecretDB{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	vault := NewVault(db, EnvMasterKeyProvider{Value: key})

	err = vault.Store(
		context.Background(),
		"spotify/main/access_token",
		"spotify",
		"super-secret-token",
	)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	var row SecretDB
	if err := db.First(&row, "id = ?", "spotify/main/access_token").Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if string(row.Ciphertext) == "super-secret-token" {
		t.Fatal("secret was stored in plaintext")
	}
	got, err := vault.Resolve(context.Background(), "spotify/main/access_token")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "super-secret-token" {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestDecodeMasterKeyRejectsWrongLength(t *testing.T) {
	if _, err := DecodeMasterKey("short"); err == nil {
		t.Fatal("expected short key to fail")
	}
}
