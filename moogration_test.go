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
	"fmt"
	"io/ioutil"
	"log"
	"runtime"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// instantiate a DB connection using test config, and create the migration table
func getTestDB(t *testing.T) (*sql.DB, func()) {
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

	// create DB if not exists
	noDBConnString := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/",
		conf["dbUser"],
		conf["dbPass"],
		conf["dbHost"],
		conf["dbPort"],
	)

	noDBConn, err := sql.Open("mysql", noDBConnString)
	if err != nil {
		t.Log("failed connecting to DB for initial setup", err)
		t.FailNow()
	}
	defer noDBConn.Close()

	sqlCreate := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", conf["dbName"])
	_, err = noDBConn.Exec(sqlCreate)
	if err != nil {
		t.Log("failed creating test database")
		t.FailNow()
	}

	connString := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s",
		conf["dbUser"],
		conf["dbPass"],
		conf["dbHost"],
		conf["dbPort"],
		conf["dbName"],
	)

	conn, err := sql.Open("mysql", connString)
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
		sqlDrop := fmt.Sprintf("DROP DATABASE IF EXISTS %s", conf["dbName"])
		conn.Exec(sqlDrop)
	}

	return conn, teardown
}

func assertOk(t *testing.T, err error) {
	if err != nil {
		t.Logf("Unexpected error in test: %s", err.Error())
		t.FailNow()
	}
}

const assertionStackFrames = 3

func assertEquals(t *testing.T, exp interface{}, actual interface{}) {
	if exp != actual {
		stack := make([]uintptr, assertionStackFrames)
		stackTrace := ""
		_ = runtime.Callers(1, stack)
		for _, frame := range stack {
			fn := runtime.FuncForPC(frame)
			if fn == nil {
				break
			}
			file, line := fn.FileLine(frame)
			stackTrace += fmt.Sprintf("\n[%s] %s:%d", fn.Name(), file, line)
		}

		t.Logf(
			"Assertion failed:\nexpected %v (type %T)\ngot %v (type %T)",
			exp,
			exp,
			actual,
			actual,
		)
		t.Log(stackTrace)
		t.FailNow()
	}
}

func TestMigrationStatus(t *testing.T) {
	db, teardown := getTestDB(t)
	defer teardown()

	testMigration := &Migration{
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
}

func TestMigrationLatestBatch(t *testing.T) {
	db, teardown := getTestDB(t)
	defer teardown()
	initialLatestBatch, err := latestBatch(db)
	assertOk(t, err)
	assertEquals(t, 0, initialLatestBatch)

	testMigration := &Migration{
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

func TestRollback(t *testing.T) {
	db, _ := getTestDB(t)
	// defer teardown()

	testMigration1 := &Migration{
		Name: "001_test_migration1",
		Up: `CREATE TABLE IF NOT EXISTS test_table1 (
				id int UNSIGNED NOT NULL AUTO_INCREMENT,
				string VARCHAR(255),
				PRIMARY KEY (id)
			);`,
		Down: `DROP TABLE IF EXISTS test_table1;`,
	}

	testMigration2 := &Migration{
		Name: "002_test_migration2",
		Up: `CREATE TABLE IF NOT EXISTS test_table1 (
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
