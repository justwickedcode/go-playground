package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"quotes-crawler/internal/db"
	"quotes-crawler/internal/fetcher"
	"quotes-crawler/internal/models"
	"quotes-crawler/internal/parser"
	"strconv"

	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	// postgres
	pool, err := db.ConnectPostgres(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Could not connect to Postgres: ", err)
	}
	log.Println("Connected to Postgres!")

	if err = db.Migrate(pool); err != nil {
		log.Fatal("Could not migrate DB: ", err)
	}
	log.Println("Migrated database!")

	// redis
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		log.Fatal("Invalid REDIS_DB value: ", err)
	}

	redisClient, err := db.ConnectRedis(os.Getenv("REDIS_ADDR"), os.Getenv("REDIS_PASSWORD"), redisDB)
	if err != nil {
		log.Fatal("Could not connect to Redis: ", err)
	}
	log.Println("Connected to Redis!")

	// store
	store := db.NewStore(pool, redisClient)

	if err := store.WarmSimhashCache(ctx); err != nil {
		log.Fatal("Could not warm simhash cache: ", err)
	}
	log.Println("Warmed simhash cache!")

	// ---- fetch and parse ----
	var allQuotes []models.Quote
	for i := 1; i <= 10; i++ {
		url := fmt.Sprintf("https://quotes.toscrape.com/page/%d/", i)
		html, err := fetcher.Fetch(url)
		if err != nil {
			log.Printf("Could not fetch page %d: %v", i, err)
			continue
		}

		quotes, err := (&parser.ToscrapeParser{}).Parse(html)
		if err != nil {
			log.Printf("Could not parse page %d: %v", i, err)
			continue
		}

		allQuotes = append(allQuotes, quotes...)
	}

	// ---- save all ----
	var saved, skipped, total int
	for _, quote := range allQuotes {
		total++
		inserted, err := store.SaveQuote(ctx, quote)
		if err != nil {
			log.Printf("Could not save quote: %v", err)
			continue
		}
		if inserted {
			saved++
		} else {
			skipped++
		}
	}

	log.Printf("Saved %d/%d quotes! (%d duplicates skipped)", saved, total, skipped)
}
