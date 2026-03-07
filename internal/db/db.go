/*
Madexchanger — HTTP/HTTPS email relay proxy for Madmail.
Copyright © 2024 The Mad Org contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// Package db provides SQLite-backed storage for relay statistics,
// route counters, endpoint rewrite rules, and relay filter rules.
//
// Instead of tracking individual messages, it records:
//   - Routes: from_server → to_server with a message count
//   - Aggregate stats: total relayed, rejected, errors, bytes
package db

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database with typed access methods for
// route stats, rewrite rules, and relay filters.
type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

// Route tracks the message count between two servers.
type Route struct {
	ID         int64     `json:"id"`
	FromServer string    `json:"from_server"` // Source server domain/IP.
	ToServer   string    `json:"to_server"`   // Destination server domain/IP.
	Count      int64     `json:"count"`       // Number of messages relayed.
	LastSeen   time.Time `json:"last_seen"`   // Timestamp of last relay.
}

// RewriteRule defines an endpoint rewrite transformation.
// When a message matches the Pattern (applied to the Field),
// the matched value is replaced with Replacement.
type RewriteRule struct {
	ID          int64  `json:"id"`
	Enabled     bool   `json:"enabled"`
	Field       string `json:"field"`       // "mail_from", "mail_to", "downstream".
	Pattern     string `json:"pattern"`     // Exact match or glob pattern.
	Replacement string `json:"replacement"` // Replacement value.
	Comment     string `json:"comment"`     // Human-readable description.
}

// RelayFilter defines domain/address-level relay rules for
// "relay selected" mode. Messages matching a filter are relayed;
// everything else is rejected.
type RelayFilter struct {
	ID      int64  `json:"id"`
	Enabled bool   `json:"enabled"`
	Field   string `json:"field"`   // "mail_from", "mail_to", "domain".
	Pattern string `json:"pattern"` // Exact match or wildcard.
	Comment string `json:"comment"`
}

// Stats holds aggregate relay statistics.
type Stats struct {
	TotalRelayed  int64 `json:"total_relayed"`
	TotalRejected int64 `json:"total_rejected"`
	TotalErrors   int64 `json:"total_errors"`
	TotalBytes    int64 `json:"total_bytes"`
}

// Open creates or opens the SQLite database at the given path
// and initializes all required tables.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("db: failed to open %s: %w", path, err)
	}

	// Set pragmas for performance.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-8000", // 8 MB cache.
	} {
		if _, err := sqlDB.Exec(pragma); err != nil {
			return nil, fmt.Errorf("db: pragma %q failed: %w", pragma, err)
		}
	}

	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

// Close releases the database resources.
func (d *DB) Close() error {
	return d.db.Close()
}

// migrate creates all tables if they don't already exist.
func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS routes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		from_server TEXT NOT NULL DEFAULT '',
		to_server   TEXT NOT NULL DEFAULT '',
		count       INTEGER NOT NULL DEFAULT 0,
		last_seen   DATETIME NOT NULL DEFAULT (datetime('now')),
		UNIQUE(from_server, to_server)
	);

	CREATE TABLE IF NOT EXISTS stats (
		id             INTEGER PRIMARY KEY CHECK (id = 1),
		total_relayed  INTEGER NOT NULL DEFAULT 0,
		total_rejected INTEGER NOT NULL DEFAULT 0,
		total_errors   INTEGER NOT NULL DEFAULT 0,
		total_bytes    INTEGER NOT NULL DEFAULT 0
	);
	INSERT OR IGNORE INTO stats (id) VALUES (1);

	CREATE TABLE IF NOT EXISTS rewrite_rules (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		enabled     INTEGER NOT NULL DEFAULT 1,
		field       TEXT NOT NULL DEFAULT '',
		pattern     TEXT NOT NULL DEFAULT '',
		replacement TEXT NOT NULL DEFAULT '',
		comment     TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS relay_filters (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		enabled INTEGER NOT NULL DEFAULT 1,
		field   TEXT NOT NULL DEFAULT '',
		pattern TEXT NOT NULL DEFAULT '',
		comment TEXT NOT NULL DEFAULT ''
	);
	`
	_, err := d.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("db: migration failed: %w", err)
	}
	return nil
}

// --- Route & Stats ---

// RecordRelay increments the route counter for from→to and updates
// the aggregate stats (relayed count + bytes).
func (d *DB) RecordRelay(fromServer, toServer string, sizeBytes int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Upsert route counter.
	_, err = tx.Exec(`
		INSERT INTO routes (from_server, to_server, count, last_seen)
		VALUES (?, ?, 1, datetime('now'))
		ON CONFLICT(from_server, to_server) DO UPDATE SET
			count = count + 1,
			last_seen = datetime('now')`,
		fromServer, toServer)
	if err != nil {
		return err
	}

	// Increment aggregate stats.
	_, err = tx.Exec(`UPDATE stats SET total_relayed = total_relayed + 1, total_bytes = total_bytes + ? WHERE id = 1`,
		sizeBytes)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// RecordRejected increments the rejected counter.
func (d *DB) RecordRejected() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE stats SET total_rejected = total_rejected + 1 WHERE id = 1`)
	return err
}

// RecordError increments the error counter.
func (d *DB) RecordError() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE stats SET total_errors = total_errors + 1 WHERE id = 1`)
	return err
}

// GetStats returns aggregate relay statistics.
func (d *DB) GetStats() (*Stats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &Stats{}
	err := d.db.QueryRow(`SELECT total_relayed, total_rejected, total_errors, total_bytes FROM stats WHERE id = 1`).
		Scan(&stats.TotalRelayed, &stats.TotalRejected, &stats.TotalErrors, &stats.TotalBytes)
	return stats, err
}

// ListRoutes returns all known routes (from_server → to_server) with counts.
func (d *DB) ListRoutes() ([]Route, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, from_server, to_server, count, last_seen FROM routes ORDER BY count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var r Route
		var ts string
		if err := rows.Scan(&r.ID, &r.FromServer, &r.ToServer, &r.Count, &ts); err != nil {
			return nil, err
		}
		r.LastSeen, _ = time.Parse("2006-01-02 15:04:05", ts)
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

// --- Rewrite Rules ---

// ListRewriteRules returns all rewrite rules.
func (d *DB) ListRewriteRules() ([]RewriteRule, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, enabled, field, pattern, replacement, comment FROM rewrite_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []RewriteRule
	for rows.Next() {
		var r RewriteRule
		var enabled int
		if err := rows.Scan(&r.ID, &enabled, &r.Field, &r.Pattern, &r.Replacement, &r.Comment); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// AddRewriteRule inserts a new rewrite rule.
func (d *DB) AddRewriteRule(r *RewriteRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	res, err := d.db.Exec(`INSERT INTO rewrite_rules (enabled, field, pattern, replacement, comment) VALUES (?, ?, ?, ?, ?)`,
		enabled, r.Field, r.Pattern, r.Replacement, r.Comment)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

// UpdateRewriteRule updates an existing rewrite rule by ID.
func (d *DB) UpdateRewriteRule(r *RewriteRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := d.db.Exec(`UPDATE rewrite_rules SET enabled=?, field=?, pattern=?, replacement=?, comment=? WHERE id=?`,
		enabled, r.Field, r.Pattern, r.Replacement, r.Comment, r.ID)
	return err
}

// DeleteRewriteRule removes a rewrite rule by ID.
func (d *DB) DeleteRewriteRule(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM rewrite_rules WHERE id=?`, id)
	return err
}

// --- Relay Filters ---

// ListRelayFilters returns all relay filters.
func (d *DB) ListRelayFilters() ([]RelayFilter, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`SELECT id, enabled, field, pattern, comment FROM relay_filters ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filters []RelayFilter
	for rows.Next() {
		var f RelayFilter
		var enabled int
		if err := rows.Scan(&f.ID, &enabled, &f.Field, &f.Pattern, &f.Comment); err != nil {
			return nil, err
		}
		f.Enabled = enabled != 0
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

// AddRelayFilter inserts a new relay filter.
func (d *DB) AddRelayFilter(f *RelayFilter) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	enabled := 0
	if f.Enabled {
		enabled = 1
	}
	res, err := d.db.Exec(`INSERT INTO relay_filters (enabled, field, pattern, comment) VALUES (?, ?, ?, ?)`,
		enabled, f.Field, f.Pattern, f.Comment)
	if err != nil {
		return err
	}
	f.ID, _ = res.LastInsertId()
	return nil
}

// UpdateRelayFilter updates an existing relay filter by ID.
func (d *DB) UpdateRelayFilter(f *RelayFilter) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	enabled := 0
	if f.Enabled {
		enabled = 1
	}
	_, err := d.db.Exec(`UPDATE relay_filters SET enabled=?, field=?, pattern=?, comment=? WHERE id=?`,
		enabled, f.Field, f.Pattern, f.Comment, f.ID)
	return err
}

// DeleteRelayFilter removes a relay filter by ID.
func (d *DB) DeleteRelayFilter(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM relay_filters WHERE id=?`, id)
	return err
}
