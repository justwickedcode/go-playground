package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"quote-crawler/internal/db"
	"quote-crawler/internal/dedup"
	"quote-crawler/internal/fetcher"
	"quote-crawler/internal/models"
	"quote-crawler/internal/parser"

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
	err = db.Migrate(pool)
	if err != nil {
		log.Fatal("Could not migrate DB: ", err)
	}
	log.Println("Migrated database successfully!")

	var allQuotes []models.Quote

	for i := 1; i <= 10; i++ {
		url := fmt.Sprintf("https://quotes.toscrape.com/?q=%d", i)
		html, err := fetcher.Fetch(url)
		if err != nil {
			log.Fatal("Could not fetch page: ", err)
		}

		p := &parser.ToscrapeParser{}
		quotes, err := p.Parse(html)
		if err != nil {
			log.Fatal("Could not parse quotes: ", err)
		}
		allQuotes = append(allQuotes, quotes...)
	}

	//data, err := json.MarshalIndent(allQuotes, "", "  ")
	//if err != nil {
	//	log.Fatal("Could not marshal quotes: ", err)
	//}
	//log.Println(string(data))
	log.Println("Simhash: ", dedup.Simhash(allQuotes[0].Text))
	log.Println("SHA256: ", dedup.SHA256(allQuotes[0].Text))
	log.Println("Normalized: ", dedup.Normalize(allQuotes[0].Text))

	hash1 := dedup.Simhash(dedup.Normalize("To be or not to be"))
	hash2 := dedup.Simhash(dedup.Normalize("To be or not to be!"))
	distance := dedup.HammingDistance(hash1, hash2)
	log.Println("Hamming Distance: ", distance)
}
