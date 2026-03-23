package modules

import (
	"testing"
)

func TestOpenDB_InMemory_NilMigrationFS(t *testing.T) {
	db, err := OpenDB(":memory:", MigrationFS{})
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	// Verify the database is usable by running a simple query.
	var result int
	if err := db.QueryRow("SELECT 1").Scan(&result); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("SELECT 1 = %d, want 1", result)
	}
}

func TestDefaultDBPath_NonEmpty(t *testing.T) {
	path := DefaultDBPath()
	if path == "" {
		t.Error("DefaultDBPath returned empty string")
	}
}
