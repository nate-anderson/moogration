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
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// instantiate a DB connection using test config, and create the migration table
func getTestDB(t *testing.T) *sql.DB {
	conf := make(map[string]string, 5)
	confBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		t.Log("failed reading config.json")
		t.FailNow()
	}

	err = json.Unmarshal(confBytes, &conf)
	if err != nil {
		t.Log("failed parsing configuration json")
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
		t.Log("failed connecting to configured database")
		t.FailNow()
	}

	err = createMigrationTable(conn)
	if err != nil {
		t.Log("failed creating migration table")
		t.FailNow()
	}

	return conn
}

func assertOk(t *testing.T, err error) {
	if err != nil {
		t.Logf("Unexpected error in test: %s", err.Error())
		t.Fail()
	}
}

func assertEquals(t *testing.T, exp interface{}, actual interface{}) {
	if exp != actual {
		t.Logf(
			"Assertion failed:\nexpected %v (type %T)\ngot %v (type %T)",
			exp,
			exp,
			actual,
			actual,
		)
	}
}

func TestMigrationStatus(t *testing.T) {
	db := getTestDB(t)
	testLogger := log.New(os.Stdout, "", log.Flags())

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

	// run migration, check that hasRun flips
	testMigration.run(false, db, testLogger)

	hasRun, hasChanged = testMigration.migrationStatus(db)
	assertEquals(t, true, hasRun)
	assertEquals(t, false, hasChanged)

	// alter migration, check that hasChanged flips
	testMigration.Down = "DROP TABLE test_table;"

	hasRun, hasChanged = testMigration.migrationStatus(db)
	assertEquals(t, true, hasRun)
	assertEquals(t, true, hasChanged)

	// run down migration
	testMigration.run(true, db, testLogger)
	hasRun, hasChanged = testMigration.migrationStatus(db)
	assertEquals(t, false, hasRun)
}
