package modules

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// DefaultDBPath returns the default database file path next to the executable.
func DefaultDBPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "devgatewaydns.db"
	}
	return filepath.Join(filepath.Dir(exe), "devgatewaydns.db")
}

// OpenDB opens the SQLite database with WAL mode and runs migrations.
func OpenDB(dbPath string, migrationsFS MigrationFS) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if migrationsFS.FS != nil {
		goose.SetBaseFS(migrationsFS.FS)
		if err := goose.SetDialect("sqlite3"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set goose dialect: %w", err)
		}
		beforeVer, _ := goose.GetDBVersion(db)
		goose.SetVerbose(false)
		if err := goose.Up(db, migrationsFS.Dir); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		afterVer, _ := goose.GetDBVersion(db)
		if afterVer > beforeVer {
			log.Printf("Database migrated: version %d -> %d", beforeVer, afterVer)
		}
	}

	return db, nil
}

// VacuumDB runs VACUUM on the database to reclaim unused space.
func VacuumDB(db *sql.DB) error {
	if _, err := db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	log.Println("Database vacuum completed")
	return nil
}
