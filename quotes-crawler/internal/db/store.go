package db

import (
	"context"
	"encoding/json"
	"quotes-crawler/internal/dedup"
	"quotes-crawler/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SaveQuote(ctx context.Context, pool *pgxpool.Pool, quote models.Quote) error {
	// normalize text before hashing to ensure consistent dedup
	normalizedText := dedup.Normalize(quote.Text)

	// compute hashes from normalized text
	sha256Hash := dedup.SHA256(normalizedText)
	simhash := dedup.Simhash(normalizedText)

	// marshal tags slice to JSONB for Postgres
	tagsJSON, err := json.Marshal(quote.Tags)
	if err != nil {
		return err
	}

	// insert quote, silently skip if exact duplicate (same sha256)
	_, err = pool.Exec(ctx,
		`INSERT INTO quotes (text, author, tags, source, sha256_hash, simhash)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (sha256_hash) DO NOTHING`,
		quote.Text, quote.Author, tagsJSON, quote.Source, sha256Hash, simhash,
	)
	return err
}
