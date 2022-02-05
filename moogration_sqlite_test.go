package moogration

/*
To run tests against a database, create a `config.json` file in
the root directory with the connection details using the following
object structure

{
	"dbHost": "...",
	"dbName": "...",
	"dbPort": "...",
	"dbUser": "...",
	"dbPass": "..."
}
*/

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

// instantiate a DB connection using test config, and create the migration table
func getTestSQLiteDB(t *testing.T, name string) (*sql.DB, func()) {
	UseSQLite()
	conf := make(map[string]string, 5)
	confBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		t.Log("failed reading config.json", err)
		t.FailNow()
	}

	err = json.Unmarshal(confBytes, &conf)
	if err != nil {
		t.Log("failed parsing configuration json", err)
		t.FailNow()
	}

	conn, err := sql.Open("sqlite", name)
	if err != nil {
		t.Log("failed connecting to configured database", err)
		t.FailNow()
	}

	err = createMigrationTable(conn)
	if err != nil {
		t.Log("failed creating migration table", err)
		t.FailNow()
	}

	teardown := func() {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Failed to close test DB: %s", err.Error())
		}
		err = os.Remove(name)
		if err != nil {
			log.Fatalf("Failed to teardown test DB: %s", err.Error())
		}
	}

	return conn, teardown
}

func TestSQLiteMigrationStatus(t *testing.T) {
	db, teardown := getTestSQLiteDB(t, "migration_status_test")
	defer teardown()

	testMigration := &Migration{
		Name: "001_test_migration",
		Up: `CREATE TABLE IF NOT EXISTS test_table (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			string TEXT
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
}

func TestMigrationLatestBatch(t *testing.T) {
	db, teardown := getTestSQLiteDB(t, "latest_batch_test")
	defer teardown()
	initialLatestBatch, err := latestBatch(db)
	assertOk(t, err)
	assertEquals(t, 0, initialLatestBatch)

	testMigration := &Migration{
		Name: "001_test_migration",
		Up: `CREATE TABLE IF NOT EXISTS test_table (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				string TEXT
			);`,
		Down: `DROP TABLE IF EXISTS test_table;`,
	}

	Register(testMigration)

	RunLatest(db, false, false, log.Default())

	afterLatestBatch, err := latestBatch(db)
	assertOk(t, err)
	assertEquals(t, 1, afterLatestBatch)
}

func TestSQLiteRollback(t *testing.T) {
	db, teardown := getTestSQLiteDB(t, "rollback_test")
	defer teardown()

	testMigration1 := &Migration{
		Name: "001_test_migration1",
		Up: `CREATE TABLE IF NOT EXISTS test_table1 (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				string TEXT
			);`,
		Down: `DROP TABLE IF EXISTS test_table1;`,
	}

	testMigration2 := &Migration{
		Name: "002_test_migration2",
		Up: `CREATE TABLE IF NOT EXISTS test_table2 (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				string TEXT
			);`,
		Down: `DROP TABLE IF EXISTS test_table2;`,
	}

	Register(testMigration1, testMigration2)

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
