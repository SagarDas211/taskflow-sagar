package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load .env file
	_ = godotenv.Load(".env", "../.env")
	var (
		databaseURL = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
		path        = flag.String("path", "db/migrations", "path to migration files")
		direction   = flag.String("direction", "up", "migration direction: up or down")
		steps       = flag.Int("steps", 0, "number of steps for up/down; 0 means all up or one down")
	)

	flag.Parse()

	if *databaseURL == "" {
		log.Fatal("database connection string is required via -database-url or DATABASE_URL")
	}

	sourceURL := fmt.Sprintf("file://%s", *path)

	m, err := migrate.New(sourceURL, *databaseURL)
	if err != nil {
		log.Fatalf("create migrate instance: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("close source: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("close database: %v", dbErr)
		}
	}()

	switch *direction {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Steps(-1)
		}
	default:
		log.Fatalf("invalid direction %q: use up or down", *direction)
	}

	if err != nil && err != migrate.ErrNoChange {
		log.Fatalf("run migrations: %v", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("no migration changes to apply")
		return
	}

	log.Printf("migration %s completed successfully", *direction)
}
