package example

import (
	"github.com/nate-anderson/moogration"
)

var userTableMigration = moogration.Migration{
	Name: "001_create_table_user",
	Up: `CREATE TABLE user (
		id bigint NOT NULL AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(255) UNIQUE,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP	
	);`,
	Down: `DROP TABLE user;`
}

var postTableMigration = moogration.Migration{
	Name: "002_create_table_post",
	Up: `CREATE TABLE post (
		...
	);`,
	Down: `DROP TABLE post;`,
}

func main() {
	moogration.Register(&usersTableMigration, &postTableMigration)
}