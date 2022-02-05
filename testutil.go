package moogration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"testing"
)

/*
To run tests against a MySQL database, create a `config.json` file in
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

// instantiate a DB connection using test config, and create the migration table
func getTestMySQLDB(t *testing.T) (*sql.DB, func()) {
	UseMySQL()
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
		t.Logf("failed creating test database: %s", err.Error())
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
