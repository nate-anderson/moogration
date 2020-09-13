package example

import (
	"github.com/nate-anderson/moogration"
)

var usersTableMigration = moogration.Migration{
	Name: "001_create_table_user",
	Up: `CREATE TABLE user (
		id bigint NOT NULL AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(255) UNIQUE,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP	
	);`,
	Down: `DROP TABLE user;`
}

func main() {
	moogration.RegisterMigration(&usersTableMigration)
}