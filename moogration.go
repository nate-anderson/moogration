package moogration

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
)

// Migration contains the up and down SQL of a migration, as well as a name.
// The name is used as a sorting key, so the recommended format is 001_migration_name
type Migration struct {
	Up   string
	Down string
	Name string
}

var registeredMigrations = []*Migration{}

// RegisterMigration registers a migration to be run by RunLatest
func RegisterMigration(m *Migration) {
	registeredMigrations = append(registeredMigrations, m)
}

// this table tracks migratipn statuses
const createMigrationTableSQL = `
	CREATE TABLE IF NOT EXISTS migration (
		id int NOT NULL AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255),
		sql_hash VARCHAR(255),
		migrated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
`

// hashes are stored to safety check that migrations have not been edited
// since they were run
func (m Migration) hash() string {
	data := []byte(m.Up + m.Down)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func (m Migration) migrationStatus(db *sql.DB) (hasRun, hasChanged bool) {
	dbMigration := Migration{}
	var dbHash string
	query := "SELECT name, sql_hash FROM migration WHERE name = ?"
	migration := db.QueryRow(query, m.Name)

	err := migration.Scan(&dbMigration.Name, &dbHash)
	if err != nil {
		if err == sql.ErrNoRows {
			// hasRun defaults to false, hasChanged is vacuously false
			return
		}
		panic(err)
	}

	// if no ErrNoRows, the migration has run
	hasRun = true

	// check if migration has changed since run
	if dbHash != m.hash() {
		hasChanged = true
	}
	return
}

func (m Migration) setMigrationStatus(down bool, db *sql.DB) {
	if down {
		stmt := "DELETE FROM migration WHERE name = ?"
		_, err := db.Exec(stmt, m.Name)
		if err != nil {
			err = fmt.Errorf("error deleting migration record for migration '%s': %w", m.Name, err)
			panic(err)
		}
		return
	}
	stmt := "INSERT INTO migration (name, sql_hash) VALUES (?, ?)"
	_, err := db.Exec(stmt, m.Name, m.hash())
	if err != nil {
		err = fmt.Errorf("error inserting migration record for migration '%s': %w", m.Name, err)
		panic(err)
	}
}

// run a migration on the provided connection
func (m Migration) run(down bool, db *sql.DB) {
	if down {
		log.Printf("migrate :: DOWN :: %s", m.Name)
		_, err := db.Exec(m.Down)
		if err != nil {
			err = fmt.Errorf("error running migration '%s' (DOWN): %w", m.Name, err)
			panic(err)
		}
	} else {
		log.Printf("migrate :: UP :: %s", m.Name)
		_, err := db.Exec(m.Up)
		if err != nil {
			err = fmt.Errorf("error running migration '%s' (UP): %w", m.Name, err)
			panic(err)
		}
	}
}

// RunLatest runs all migrations that have not been run since the last migration
func RunLatest(db *sql.DB, down, force bool) {
	// create migrations table if not exist
	_, err := db.Exec(createMigrationTableSQL)
	if err != nil {
		// wrap error with some context
		err = fmt.Errorf("error running create migration table migration: %w", err)
		panic(err)
	}

	// sort migrations to run in order of creation
	sort.Slice(registeredMigrations, func(i, j int) bool {
		// if running down migrations, sort descending
		if down {
			return registeredMigrations[i].Name > registeredMigrations[j].Name
		}
		return registeredMigrations[i].Name < registeredMigrations[j].Name

	})

	for _, m := range registeredMigrations {
		// check if migration has been run or changed
		hasRun, hasChanged := m.migrationStatus(db)
		if hasRun {
			if hasChanged && !force {
				err := fmt.Errorf("previously run migration '%s' has changed since run", m.Name)
				panic(err)
			}
			if !force && !down {
				continue
			}
		}

		m.run(down, db)
		m.setMigrationStatus(down, db)
	}
}