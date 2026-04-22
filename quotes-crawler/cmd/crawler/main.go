package main

import (
	"context"
	"log"
	"os"
	"quote-crawler/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	dbUrl := os.Getenv("DATABASE_URL")
	pool, err := db.Connect(dbUrl)
	if err != nil {
		log.Fatal("Could not establish connection to the database: ", err)
	}

	err = pool.Ping(context.Background())
	if err != nil {
		log.Fatal("Could not ping database: ", err)
	}

	log.Println("Connected to database successfully!")
}
