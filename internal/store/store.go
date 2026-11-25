// Package store keeps all mcpDeck state in one SQLite database (RUN-3).
//
// Config entities (APIs, requests, toolkits, tools, agents...) are stored as
// JSON documents next to the columns used for lookups. Calls and the audit
// log have real columns because they are filtered and aggregated.
package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

//go:embed migrations/*.sql
var migrations embed.FS

// ErrNotFound is returned when a row doesn't exist.
var ErrNotFound = errors.New("not found")

// Store is the SQLite database.
type Store struct{ DB *sql.DB }

// Open opens (creating if needed) mcpdeck.db in dir and migrates it.
func Open(ctx context.Context, dir string) (*Store, error) {
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, "mcpdeck.db")) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migration (name TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var n int
		if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM schema_migration WHERE name = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, _ := migrations.ReadFile("migrations/" + name)
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(body)) {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migration(name) VALUES (?)`, name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// splitSQL splits on semicolons at line ends, keeping trigger bodies whole.
func splitSQL(body string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		up := strings.ToUpper(t)
		if strings.Contains(up, " BEGIN ") || strings.HasSuffix(up, " BEGIN") {
			depth++
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(up, "END;") && depth > 0 {
			depth--
		}
		if depth == 0 && strings.HasSuffix(t, ";") {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	return out
}

// Put inserts or replaces a document. cols are the extra indexed columns.
func (s *Store) Put(ctx context.Context, table, id string, v any, cols map[string]any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	names := []string{"id", "data"}
	args := []any{id, string(data)}
	keys := make([]string, 0, len(cols))
	for k := range cols {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		names = append(names, k)
		args = append(args, cols[k])
	}
	q := fmt.Sprintf(`INSERT OR REPLACE INTO %s (%s) VALUES (%s)`, table, strings.Join(names, ","),
		strings.TrimSuffix(strings.Repeat("?,", len(names)), ","))
	if _, err := s.DB.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("save %s: %w", table, err)
	}
	return nil
}

// Delete removes a document.
func (s *Store) Delete(ctx context.Context, table, id string) error {
	_, err := s.DB.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table), id)
	return err
}

// Get loads one document by id.
func Get[T any](ctx context.Context, s *Store, table, id string) (T, error) {
	var v T
	var data string
	err := s.DB.QueryRowContext(ctx, fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, table), id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return v, fmt.Errorf("%s %s: %w", table, id, ErrNotFound)
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(data), &v)
	return v, err
}

// Find loads the first document matching where.
func Find[T any](ctx context.Context, s *Store, table, where string, args ...any) (T, error) {
	var zero T
	list, err := List[T](ctx, s, table, where+" LIMIT 1", args...)
	if err != nil {
		return zero, err
	}
	if len(list) == 0 {
		return zero, fmt.Errorf("%s: %w", table, ErrNotFound)
	}
	return list[0], nil
}

// List loads documents matching where (may be empty, may include ORDER BY).
func List[T any](ctx context.Context, s *Store, table, where string, args ...any) ([]T, error) {
	q := fmt.Sprintf(`SELECT data FROM %s`, table)
	if strings.TrimSpace(where) != "" {
		if !strings.HasPrefix(strings.TrimSpace(strings.ToUpper(where)), "ORDER") && !strings.HasPrefix(strings.TrimSpace(strings.ToUpper(where)), "LIMIT") {
			q += " WHERE"
		}
		q += " " + where
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", table, err)
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Setting returns a setting or def.
func (s *Store) Setting(ctx context.Context, key, def string) string {
	var v string
	if err := s.DB.QueryRowContext(ctx, `SELECT value FROM setting WHERE key = ?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

// SetSetting stores a setting.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT OR REPLACE INTO setting(key, value) VALUES (?, ?)`, key, value)
	return err
}

// Settings returns every stored setting.
func (s *Store) Settings(ctx context.Context) (map[string]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT key, value FROM setting`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
