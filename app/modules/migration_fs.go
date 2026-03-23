package modules

import "io/fs"

// MigrationFS holds the embedded filesystem for database migrations.
type MigrationFS struct {
	FS  fs.FS
	Dir string
}
