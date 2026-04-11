package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed sql/**/*.sql
var migrationFS embed.FS

var migrationTargets = map[string]string{
	"users_db":        "sql/users",
	"wallets_db":      "sql/wallets",
	"transactions_db": "sql/transactions",
}

func RunMigrations(ctx context.Context, cfg *Config) error {
	adminDB, err := sql.Open("postgres", cfg.DSN("postgres"))
	if err != nil {
		return fmt.Errorf("open admin connection: %w", err)
	}
	defer adminDB.Close()

	if err := pingDB(ctx, adminDB); err != nil {
		return fmt.Errorf("ping admin connection: %w", err)
	}

	for databaseName, migrationDir := range migrationTargets {
		if err := ensureDatabase(ctx, adminDB, databaseName); err != nil {
			return fmt.Errorf("ensure database %s: %w", databaseName, err)
		}

		if err := applyDatabaseMigrations(ctx, cfg, databaseName, migrationDir); err != nil {
			return fmt.Errorf("apply migrations for %s: %w", databaseName, err)
		}
	}

	return nil
}

func pingDB(ctx context.Context, db *sql.DB) error {
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return db.PingContext(pingCtx)
}

func ensureDatabase(ctx context.Context, adminDB *sql.DB, databaseName string) error {
	var exists bool
	if err := adminDB.QueryRowContext(
		ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`,
		databaseName,
	).Scan(&exists); err != nil {
		return err
	}

	if exists {
		log.Printf("database %s already exists", databaseName)
		return nil
	}

	query := fmt.Sprintf(`CREATE DATABASE %s`, quoteIdentifier(databaseName))
	if _, err := adminDB.ExecContext(ctx, query); err != nil {
		return err
	}

	log.Printf("database %s created", databaseName)
	return nil
}

func applyDatabaseMigrations(ctx context.Context, cfg *Config, databaseName, migrationDir string) error {
	db, err := sql.Open("postgres", cfg.DSN(databaseName))
	if err != nil {
		return fmt.Errorf("open db connection: %w", err)
	}
	defer db.Close()

	if err := pingDB(ctx, db); err != nil {
		return fmt.Errorf("ping db connection: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrationFS, migrationDir+"/*.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, file := range files {
		if err := applyMigrationFile(ctx, db, file, cfg.StatementTimeout); err != nil {
			return err
		}
	}

	return nil
}

func applyMigrationFile(ctx context.Context, db *sql.DB, file string, statementTimeout time.Duration) error {
	version := migrationVersion(file)
	contents, err := migrationFS.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", file, err)
	}
	checksum := sha256.Sum256(contents)
	checksumHex := hex.EncodeToString(checksum[:])

	var appliedChecksum string
	err = db.QueryRowContext(
		ctx,
		`SELECT checksum FROM schema_migrations WHERE version = $1`,
		version,
	).Scan(&appliedChecksum)
	switch {
	case err == nil:
		if appliedChecksum != checksumHex {
			return fmt.Errorf("migration %s already applied with different checksum", version)
		}
		log.Printf("migration %s already applied", version)
		return nil
	case err != sql.ErrNoRows:
		return fmt.Errorf("check migration %s: %w", version, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`SET LOCAL statement_timeout = '%dms'`, statementTimeout.Milliseconds())); err != nil {
		return fmt.Errorf("set statement timeout for %s: %w", version, err)
	}

	if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
		return fmt.Errorf("exec migration %s: %w", version, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`,
		version,
		checksumHex,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}

	log.Printf("migration %s applied", version)
	return nil
}

func migrationVersion(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
