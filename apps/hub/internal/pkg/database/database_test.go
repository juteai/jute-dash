package database

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDirectoryAndMigrates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "jute.db")
	db, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if db.Path() != path || db.DB() == nil {
		t.Fatalf("unexpected database handle: path=%q db=%v", db.Path(), db.DB())
	}
	if err := db.Migrate(&databaseTestModel{}); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(" ", nil); err == nil {
		t.Fatal("Open() error = nil, want error")
	}
}

type databaseTestModel struct {
	ID string `gorm:"primaryKey"`
}
