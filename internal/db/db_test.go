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

package db

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestRecordRelayAndStats(t *testing.T) {
	d := openTestDB(t)

	// Record a few relays.
	if err := d.RecordRelay("1.1.1.1", "2.2.2.2", 1024); err != nil {
		t.Fatalf("RecordRelay error: %v", err)
	}
	if err := d.RecordRelay("1.1.1.1", "2.2.2.2", 2048); err != nil {
		t.Fatalf("RecordRelay error: %v", err)
	}
	if err := d.RecordRelay("1.1.1.1", "3.3.3.3", 512); err != nil {
		t.Fatalf("RecordRelay error: %v", err)
	}

	// Check stats.
	stats, err := d.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	if stats.TotalRelayed != 3 {
		t.Errorf("TotalRelayed = %d, want 3", stats.TotalRelayed)
	}
	if stats.TotalBytes != 3584 {
		t.Errorf("TotalBytes = %d, want 3584", stats.TotalBytes)
	}
	if stats.TotalRejected != 0 {
		t.Errorf("TotalRejected = %d, want 0", stats.TotalRejected)
	}
	if stats.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d, want 0", stats.TotalErrors)
	}
}

func TestRoutes(t *testing.T) {
	d := openTestDB(t)

	_ = d.RecordRelay("1.1.1.1", "2.2.2.2", 100)
	_ = d.RecordRelay("1.1.1.1", "2.2.2.2", 200)
	_ = d.RecordRelay("1.1.1.1", "3.3.3.3", 300)

	routes, err := d.ListRoutes()
	if err != nil {
		t.Fatalf("ListRoutes error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d, want 2", len(routes))
	}

	// Routes ordered by count DESC, so 1.1.1.1→2.2.2.2 (count=2) first.
	if routes[0].FromServer != "1.1.1.1" || routes[0].ToServer != "2.2.2.2" {
		t.Errorf("route[0] = %s→%s, want 1.1.1.1→2.2.2.2", routes[0].FromServer, routes[0].ToServer)
	}
	if routes[0].Count != 2 {
		t.Errorf("route[0].Count = %d, want 2", routes[0].Count)
	}
	if routes[1].Count != 1 {
		t.Errorf("route[1].Count = %d, want 1", routes[1].Count)
	}
}

func TestRecordRejectedAndError(t *testing.T) {
	d := openTestDB(t)

	_ = d.RecordRejected()
	_ = d.RecordRejected()
	_ = d.RecordError()

	stats, err := d.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	if stats.TotalRejected != 2 {
		t.Errorf("TotalRejected = %d, want 2", stats.TotalRejected)
	}
	if stats.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", stats.TotalErrors)
	}
}

func TestEmptyStats(t *testing.T) {
	d := openTestDB(t)

	stats, err := d.GetStats()
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	if stats.TotalRelayed != 0 || stats.TotalRejected != 0 || stats.TotalErrors != 0 || stats.TotalBytes != 0 {
		t.Errorf("empty stats should be all zeros, got %+v", stats)
	}
}

func TestRewriteRulesCRUD(t *testing.T) {
	d := openTestDB(t)

	rule := &RewriteRule{Enabled: true, Field: "mail_from", Pattern: "old@x.org", Replacement: "new@x.org", Comment: "test"}
	if err := d.AddRewriteRule(rule); err != nil {
		t.Fatalf("AddRewriteRule error: %v", err)
	}
	if rule.ID == 0 {
		t.Fatal("rule.ID should be non-zero after insert")
	}

	rules, _ := d.ListRewriteRules()
	if len(rules) != 1 {
		t.Fatalf("len(rules) = %d, want 1", len(rules))
	}

	rule.Enabled = false
	rule.Comment = "updated"
	_ = d.UpdateRewriteRule(rule)

	_ = d.DeleteRewriteRule(rule.ID)
	rules, _ = d.ListRewriteRules()
	if len(rules) != 0 {
		t.Errorf("len(rules) after delete = %d, want 0", len(rules))
	}
}

func TestRelayFiltersCRUD(t *testing.T) {
	d := openTestDB(t)

	filter := &RelayFilter{Enabled: true, Field: "domain", Pattern: "example.org", Comment: "test"}
	if err := d.AddRelayFilter(filter); err != nil {
		t.Fatalf("AddRelayFilter error: %v", err)
	}
	if filter.ID == 0 {
		t.Fatal("filter.ID should be non-zero after insert")
	}

	filters, _ := d.ListRelayFilters()
	if len(filters) != 1 {
		t.Fatalf("len(filters) = %d, want 1", len(filters))
	}

	_ = d.DeleteRelayFilter(filter.ID)
	filters, _ = d.ListRelayFilters()
	if len(filters) != 0 {
		t.Errorf("len(filters) after delete = %d, want 0", len(filters))
	}
}
