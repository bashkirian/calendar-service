// Package mysql stores calendar entries and the shared job schedule.
package mysql

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open forces parseTime and a UTC session regardless of the DSN, so NOW(3) is UTC too.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Params = map[string]string{"time_zone": "'+00:00'"}

	connector, err := driver.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("create connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectMySQL, db, fsys)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
