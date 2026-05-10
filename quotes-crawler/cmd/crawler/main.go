package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"quotes-crawler/internal/db"
	"quotes-crawler/internal/fetcher"
	"quotes-crawler/internal/models"
	"quotes-crawler/internal/parser"

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

	// ---- dedup tests ----
	log.Println("--- running dedup tests ---")

	if len(allQuotes) > 0 {
		first := allQuotes[0]

		// exact duplicate: same quote verbatim
		inserted, err := store.SaveQuote(ctx, first)
		if err != nil {
			log.Printf("exact dup test error: %v", err)
		} else if !inserted {
			log.Printf("PASS: exact duplicate correctly rejected (%q)", first.Text[:40])
		} else {
			log.Printf("FAIL: exact duplicate was inserted (%q)", first.Text[:40])
		}

		// near duplicate: same quote with minor text change
		nearDup := first
		nearDup.Text = strings.TrimRight(first.Text, ".") + "!"
		inserted, err = store.SaveQuote(ctx, nearDup)
		if err != nil {
			log.Printf("near dup test error: %v", err)
		} else if !inserted {
			log.Printf("PASS: near-duplicate correctly rejected (%q)", nearDup.Text[:40])
		} else {
			log.Printf("FAIL: near-duplicate was inserted (%q)", nearDup.Text[:40])
		}

		// genuinely new quote
		newQuote := models.Quote{
			Text:   "This is a completely unique quote that does not exist in the database at all.",
			Author: "Test Author",
			Source: "test",
		}
		inserted, err = store.SaveQuote(ctx, newQuote)
		if err != nil {
			log.Printf("new quote test error: %v", err)
		} else if inserted {
			log.Println("PASS: new quote correctly inserted")
		} else {
			log.Println("FAIL: new quote was rejected!")
		}
	}

	log.Println("--- dedup tests done ---")

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
