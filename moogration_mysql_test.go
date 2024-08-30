package moogration

import (
	"log"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestMigrationStatus(t *testing.T) {
	db, teardown := getTestMySQLDB(t)
	defer teardown()

	testMigration := Migration{
		Name: "001_test_migration",
		Up: `CREATE TABLE IF NOT EXISTS test_table (
			id int UNSIGNED NOT NULL AUTO_INCREMENT,
			string VARCHAR(255),
			PRIMARY KEY (id)
		);`,
		Down: `DROP TABLE IF EXISTS test_table;`,
	}

	hasRun, hasChanged := testMigration.migrationStatus(db)
	assertEquals(t, false, hasRun)
	assertEquals(t, false, hasChanged)

	Register(testMigration)
	RunLatest(db, false, false, log.Default())

	hasRun, hasChanged = testMigration.migrationStatus(db)
	assertEquals(t, true, hasRun)
	assertEquals(t, false, hasChanged)

	// alter migration, check that hasChanged flips
	testMigration.Down = "DROP TABLE test_table;"

	hasRun, hasChanged = testMigration.migrationStatus(db)
	assertEquals(t, true, hasRun)
	assertEquals(t, true, hasChanged)

	// run down migration
	RunLatest(db, true, false, log.Default())
	hasRun, hasChanged = testMigration.migrationStatus(db)
	assertEquals(t, false, hasRun)
	assertEquals(t, false, hasChanged)
}

func TestMySQLMigrationLatestBatch(t *testing.T) {
	db, teardown := getTestMySQLDB(t)
	defer teardown()
	initialLatestBatch, err := latestBatch(db)
	assertOk(t, err)
	assertEquals(t, 0, initialLatestBatch)

	testMigration := Migration{
		Name: "001_test_migration",
		Up: `CREATE TABLE IF NOT EXISTS test_table (
				id int UNSIGNED NOT NULL AUTO_INCREMENT,
				string VARCHAR(255),
				PRIMARY KEY (id)
			);`,
		Down: `DROP TABLE IF EXISTS test_table;`,
	}

	Register(testMigration)

	RunLatest(db, false, false, log.Default())

	afterLatestBatch, err := latestBatch(db)
	assertOk(t, err)
	assertEquals(t, 1, afterLatestBatch)
}

func TestMySQLRollback(t *testing.T) {
	db, teardown := getTestMySQLDB(t)
	defer teardown()

	testMigration1 := Migration{
		Name: "001_test_migration1",
		Up: `CREATE TABLE IF NOT EXISTS test_table1 (
				id int UNSIGNED NOT NULL AUTO_INCREMENT,
				string VARCHAR(255),
				PRIMARY KEY (id)
			);`,
		Down: `DROP TABLE IF EXISTS test_table1;`,
	}

	testMigration2 := Migration{
		Name: "002_test_migration2",
		Up: `CREATE TABLE IF NOT EXISTS test_table2 (
				id int UNSIGNED NOT NULL AUTO_INCREMENT,
				string VARCHAR(255),
				PRIMARY KEY (id)
			);`,
		Down: `DROP TABLE IF EXISTS test_table2;`,
	}

	Register(testMigration1)
	RunLatest(db, false, false, log.Default())

	Register(testMigration2)
	RunLatest(db, false, false, log.Default())

	// rollback 1
	err := Rollback(db, 1, false, log.Default())
	assertOk(t, err)

	currentBatch, err := latestBatch(db)
	assertOk(t, err)

	assertEquals(t, 1, currentBatch)

	hasRun2, _ := testMigration2.migrationStatus(db)
	assertEquals(t, false, hasRun2)

	hasRun1, _ := testMigration1.migrationStatus(db)
	assertEquals(t, true, hasRun1)
}
