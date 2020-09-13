package example

import (
	"flag"
	"log"

	"github.com/nate-anderson/moogration"
)

func main() {
	migrateFlag := flag.String("migrate", "", "run database migrations: --migrate [up|down]")
	forceMigrateFlag := flag.Bool("force", false, "force database migrations: --migrate down --force")

	if *migrateFlag != "" {
		switch *migrateFlag {
		case "up":
			moogration.RunLatest(false, *forceMigrateFlag)
			log.Println("UP migrations run successfully")
		case "down":
			moogration.RunLatest(true, *forceMigrateFlag)
			log.Println("DOWN migrations run successfully")
		default:
			log.Println("--migrate flag takes either \"up\" or \"down\"")
		}
		return
	}

	flag.PrintDefaults()
}
