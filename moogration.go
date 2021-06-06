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

// Register registers a migration to be run by RunLatest
func Register(m ...*Migration) {
	registeredMigrations = append(registeredMigrations, m...)
}

// this table tracks migratipn statuses
const createMigrationTableSQL = `
	CREATE TABLE IF NOT EXISTS migration (
		id int NOT NULL AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255),
		batch int NOT NULL,
		sql_hash VARCHAR(255),
		migrated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
`

const dropMigrationTableSQL = `DROP TABLE IF EXISTS migration;`

func createMigrationTable(db *sql.DB) error {
	_, err := db.Exec(createMigrationTableSQL)
	if err != nil {
		// wrap error with some context
		err = fmt.Errorf("error running create migration table migration: %w", err)
		return err
	}

	return nil
}

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

func (m Migration) setMigrationStatus(down bool, db *sql.DB, batch int) {
	if down {
		stmt := "DELETE FROM migration WHERE name = ?"
		_, err := db.Exec(stmt, m.Name)
		if err != nil {
			err = fmt.Errorf("error deleting migration record for migration '%s': %w", m.Name, err)
			panic(err)
		}
		return
	}
	stmt := "INSERT INTO migration (name, sql_hash, batch) VALUES (?, ?, ?)"
	_, err := db.Exec(stmt, m.Name, m.hash(), batch)
	if err != nil {
		err = fmt.Errorf("error inserting migration record for migration '%s': %w", m.Name, err)
		panic(err)
	}
}

// run a migration on the provided connection
func (m Migration) run(down bool, db *sql.DB, logger *log.Logger) error {
	if down {
		if logger != nil {
			logger.Printf("migrate :: DOWN :: %s", m.Name)
		}
		_, err := db.Exec(m.Down)
		if err != nil {
			err = fmt.Errorf("error running migration '%s' (DOWN): %w", m.Name, err)
			return err
		}
	} else {
		if logger != nil {
			logger.Printf("migrate :: UP :: %s", m.Name)
		}
		_, err := db.Exec(m.Up)
		if err != nil {
			err = fmt.Errorf("error running migration '%s' (UP): %w", m.Name, err)
			return err
		}
	}

	return nil
}

// get the most recently run batch number
func latestBatch(db *sql.DB) (int, error) {
	batch := 0
	sqlSelectLatestBatch := `SELECT MAX(batch) FROM migration`
	row := db.QueryRow(sqlSelectLatestBatch)
	err := row.Scan(&batch)
	// if no migrations have run, latestBatch = 0
	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("No batches")
			batch = 0
			err = nil
		} else {
			return 0, err
		}
	}
	return batch, err
}

// allBatches returns a slice of integer migration batch numbers, sorted descending
func allBatches(db *sql.DB) ([]int, error) {
	sqlSelectBatches := "SELECT DISTINCT batch FROM migration ORDER BY batch DESC"
	batches := []int{}
	rows, err := db.Query(sqlSelectBatches)
	if err != nil {
		return batches, err
	}
	defer rows.Close()

	for rows.Next() {
		var batch int
		err := rows.Scan(&batch)
		if err != nil {
			return batches, err
		}
		batches = append(batches, batch)
	}

	return batches, nil
}

// rollback a single identified migration batch. This function is intentionally left unexported,
// because migrations should not be rolled back out of order
func rollbackOneBatch(db *sql.DB, batchID int, force bool, logger *log.Logger) error {
	sqlGetMigrations := `SELECT name, sql_hash FROM migration WHERE batch = ?`
	rows, err := db.Query(sqlGetMigrations, batchID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, sqlHash string
		err := rows.Scan(&name, &sqlHash)
		if err != nil {
			return err
		}

		for _, migration := range registeredMigrations {
			if migration.Name == name {
				// validate that hash hasn't changed, permitting force
				if force || migration.hash() == sqlHash {
					// run down migration
					migration.run(true, db, logger)
				} else {
					err := fmt.Errorf("previously run migration '%s' has changed since run", migration.Name)
					panic(err)
				}
			}
		}
	}

	return nil
}

// Rollback rolls the last n batches of migrations
func Rollback(db *sql.DB, numBatches int, force bool, logger *log.Logger) error {
	batches, err := allBatches(db)
	if err != nil {
		return err
	}

	for i := 0; i < (numBatches - 1); i++ {
		batch := batches[i]
		err := rollbackOneBatch(db, batch, force, logger)
		if err != nil {
			return err
		}
	}

	return nil
}

// RunLatest runs all migrations that have not been run since the last migration
func RunLatest(db *sql.DB, down, force bool, logger *log.Logger) {
	err := createMigrationTable(db)
	if err != nil {
		panic(err)
	}

	lastBatch, err := latestBatch(db)
	if err != nil {
		err := fmt.Errorf("failed to determine last-run batch number: %w", err)
		panic(err)
	}

	currentBatch := lastBatch + 1

	// sort migrations to run in order of creation
	sort.Slice(registeredMigrations, func(i, j int) bool {
		// if running down migrations, sort descending
		if down {
			return registeredMigrations[i].Name > registeredMigrations[j].Name
		}
		return registeredMigrations[i].Name < registeredMigrations[j].Name

	})

	if logger != nil {
		logger.Printf("%d registered migrations", len(registeredMigrations))
	}

	for _, m := range registeredMigrations {
		// check if migration has been run or changed
		hasRun, hasChanged := m.migrationStatus(db)
		if hasRun && !down {
			continue
		}

		if hasChanged {
			if !force {
				if logger != nil {
					logger.Printf("WARNING: migration '%s' has changed since last run - migrations should not be edited for live databases!", m.Name)
				}
			}
		}

		err := m.run(down, db, logger)
		if err != nil {
			if force {
				if logger != nil {
					logger.Printf("ERROR: migration '%s' failed. '%s'", m.Name, err.Error())
				}
			} else {
				panic(err)
			}
		}
		m.setMigrationStatus(down, db, currentBatch)
	}
}
