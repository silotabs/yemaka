package heartbeat

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	Now func() time.Time
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, fmt.Errorf("heartbeat database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create heartbeat database directory: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open heartbeat sqlite database: %w", err)
	}
	store := &Store{db: db, Now: time.Now}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set heartbeat busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable heartbeat WAL: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Record(ctx context.Context, report Report) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("heartbeat store is not open")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin heartbeat record: %w", err)
	}
	defer tx.Rollback()
	for _, check := range report.Checks {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO heartbeat_checks (id, name, status, detail, checked_at)
			VALUES (?, ?, ?, ?, ?)
		`, check.ID, check.Name, check.Status, check.Detail, check.CheckedAt)
		if err != nil {
			return fmt.Errorf("record heartbeat check %s: %w", check.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit heartbeat record: %w", err)
	}
	return nil
}

func (s *Store) Latest(ctx context.Context) (Report, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, status, detail, checked_at
		FROM heartbeat_checks
		WHERE checked_at = (SELECT MAX(checked_at) FROM heartbeat_checks)
		ORDER BY name ASC
	`)
	if err != nil {
		return Report{}, fmt.Errorf("load latest heartbeat checks: %w", err)
	}
	defer rows.Close()
	report := Report{Overall: StatusOK}
	for rows.Next() {
		var check Check
		if err := rows.Scan(&check.ID, &check.Name, &check.Status, &check.Detail, &check.CheckedAt); err != nil {
			return Report{}, fmt.Errorf("scan heartbeat check: %w", err)
		}
		if report.GeneratedAt == "" {
			report.GeneratedAt = check.CheckedAt
		}
		report.Checks = append(report.Checks, check)
		report.Overall = combine(report.Overall, check.Status)
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("read heartbeat checks: %w", err)
	}
	return report, nil
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS heartbeat_checks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			checked_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_heartbeat_checked_at ON heartbeat_checks(checked_at);`,
		`CREATE INDEX IF NOT EXISTS idx_heartbeat_name ON heartbeat_checks(name);`,
	}
	for index, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply heartbeat migration %d: %w", index+1, err)
		}
	}
	return nil
}
