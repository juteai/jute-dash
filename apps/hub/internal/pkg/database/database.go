package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	db   *gorm.DB
	path string
}

func Open(dbPath string, log *slog.Logger) (*Database, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	gormConfig := &gorm.Config{}
	if log != nil {
		gormLevel := logger.Warn
		if log.Enabled(context.Background(), slog.LevelDebug) {
			gormLevel = logger.Info
		}
		gormLogger := NewSlogLogger(log)
		gormLogger.LogLevel = gormLevel
		gormConfig.Logger = gormLogger
	}

	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("open sqlite via gorm: %w", err)
	}

	d := &Database{db: db, path: dbPath}
	if err := d.configure(context.Background()); err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	return d, nil
}

func (d *Database) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *Database) DB() *gorm.DB {
	if d == nil {
		return nil
	}
	return d.db
}

func (d *Database) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *Database) Migrate(models ...any) error {
	if d == nil || d.db == nil {
		return errors.New("database is not open")
	}
	return d.db.AutoMigrate(models...)
}

func (d *Database) configure(ctx context.Context) error {
	db, err := d.db.DB()
	if err != nil {
		return fmt.Errorf("get underlying sql.DB: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	return nil
}
