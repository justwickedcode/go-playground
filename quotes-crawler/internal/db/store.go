package db

import (
	"context"
	"encoding/json"
	"fmt"
	"quotes-crawler/internal/dedup"
	"quotes-crawler/internal/models"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

func NewStore(pool *pgxpool.Pool, rdb *redis.Client) *Store {
	return &Store{pool: pool, rdb: rdb}
}

func bandKey(band int, value int64) string {
	return fmt.Sprintf("simhash:band:%d:%d", band, value)
}

// WarmSimhashCache loads all existing simhashes from Postgres into Redis LSH bands on startup
func (s *Store) WarmSimhashCache(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT simhash FROM quotes`)
	if err != nil {
		return err
	}
	defer rows.Close()

	pipe := s.rdb.Pipeline()
	for rows.Next() {
		var simhash int64
		if err := rows.Scan(&simhash); err != nil {
			return err
		}
		bands := dedup.ExtractBands(simhash)
		for i, band := range bands {
			pipe.SAdd(ctx, bandKey(i, band), simhash)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = pipe.Exec(ctx)
	return err
}

// isNearDuplicate checks LSH bands in Redis, runs Hamming only on candidates
func (s *Store) isNearDuplicate(ctx context.Context, simhash int64) (bool, error) {
	bands := dedup.ExtractBands(simhash)

	// fetch all band members in one round trip
	pipe := s.rdb.Pipeline()
	cmds := make([]*redis.StringSliceCmd, dedup.NumBands)
	for i, band := range bands {
		cmds[i] = pipe.SMembers(ctx, bandKey(i, band))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return false, err
	}

	// deduplicate candidates and check Hamming distance
	seen := make(map[int64]struct{})
	for _, cmd := range cmds {
		for _, m := range cmd.Val() {
			val, err := strconv.ParseInt(m, 10, 64)
			if err != nil {
				continue
			}
			if _, ok := seen[val]; ok {
				continue
			}
			seen[val] = struct{}{}
			if dedup.HammingDistance(simhash, val) < dedup.HammingThreshold {
				return true, nil
			}
		}
	}
	return false, nil
}

// addToSimhashCache adds a simhash to all LSH bands in Redis
func (s *Store) addToSimhashCache(ctx context.Context, simhash int64) error {
	bands := dedup.ExtractBands(simhash)
	pipe := s.rdb.Pipeline()
	for i, band := range bands {
		pipe.SAdd(ctx, bandKey(i, band), simhash)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) SaveQuote(ctx context.Context, quote models.Quote) (bool, error) {
	normalizedText := dedup.Normalize(quote.Text)
	sha256Hash := dedup.SHA256(normalizedText)
	simhash := dedup.Simhash(normalizedText)

	tagsJSON, err := json.Marshal(quote.Tags)
	if err != nil {
		return false, err
	}

	nearDup, err := s.isNearDuplicate(ctx, simhash)
	if err != nil {
		return false, err
	}
	if nearDup {
		return false, nil
	}

	tag, err := s.pool.Exec(ctx,
		`INSERT INTO quotes (text, author, tags, source, sha256_hash, simhash)
         VALUES ($1, $2, $3, $4, $5, $6)
         ON CONFLICT (sha256_hash) DO NOTHING`,
		quote.Text, quote.Author, tagsJSON, quote.Source, sha256Hash, simhash,
	)
	if err != nil {
		return false, err
	}

	if tag.RowsAffected() == 0 {
		return false, nil
	}

	return true, s.addToSimhashCache(ctx, simhash)
}
