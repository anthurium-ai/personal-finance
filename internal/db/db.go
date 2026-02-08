package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

func Open(path string) (*sql.DB, error) {
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	b, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, string(b))
	return err
}

func ensureDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	return mkdirAll(dir)
}
