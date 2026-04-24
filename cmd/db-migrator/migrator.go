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

type migrationTarget struct {
	databaseName string
	migrationDir string
}

var migrationTargets = []migrationTarget{
	{databaseName: "users_db", migrationDir: "sql/users"},
	{databaseName: "wallets_db", migrationDir: "sql/wallets"},
	{databaseName: "transactions_db", migrationDir: "sql/transactions"},
}

var compatibleMigrationChecksums = map[string]map[string]string{
	"transactions_db/001_init.sql": {
		// A failed deploy briefly shipped this checksum after adding balance columns
		// to 001_init.sql. The current 002 migration makes both schemas equivalent.
		"a35fb7e9443fef2ec1f22283682aed8860ef1b6dd29338fc3cd643e2fe1c6657": "legacy transfer balance columns in initial migration",
	},
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

	for _, target := range migrationTargets {
		if err := ensureDatabase(ctx, adminDB, target.databaseName); err != nil {
			return fmt.Errorf("ensure database %s: %w", target.databaseName, err)
		}

		if err := applyDatabaseMigrations(ctx, cfg, target.databaseName, target.migrationDir); err != nil {
			return fmt.Errorf("apply migrations for %s: %w", target.databaseName, err)
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
		if err := applyMigrationFile(ctx, db, databaseName, file, cfg.StatementTimeout); err != nil {
			return err
		}
	}

	return nil
}

func applyMigrationFile(ctx context.Context, db *sql.DB, databaseName, file string, statementTimeout time.Duration) error {
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
		if appliedChecksum == checksumHex {
			log.Printf("migration %s already applied", version)
			return nil
		}
		if reason, ok := compatibleAppliedChecksum(databaseName, version, appliedChecksum); ok {
			log.Printf("migration %s already applied with compatible checksum: %s", version, reason)
			return nil
		}
		return fmt.Errorf("migration %s already applied with different checksum: applied=%s current=%s", version, appliedChecksum, checksumHex)
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

func compatibleAppliedChecksum(databaseName, version, checksum string) (string, bool) {
	compatibleChecksums, ok := compatibleMigrationChecksums[databaseName+"/"+version]
	if !ok {
		return "", false
	}
	reason, ok := compatibleChecksums[checksum]
	return reason, ok
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
