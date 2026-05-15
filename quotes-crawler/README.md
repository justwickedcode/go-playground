# quotes-crawler

A scalable web crawler that collects quotes from multiple sources and stores them in PostgreSQL.

## Sources

| Source | Method | Status | Notes |
|---|---|---|---|
| [quotes.toscrape.com](https://quotes.toscrape.com) | Crawler | ✅ Done | Sandbox site, 100 quotes |
| [Quotable API](https://api.quotable.io) | API Fetcher | ⬜ Planned | 5000+ curated quotes, no auth needed |
| [BrainyQuote](https://brainyquote.com) | Crawler | ⬜ Planned | 100k+ quotes, clean HTML |
| [Wikiquote](https://en.wikiquote.org) | Crawler (MediaWiki API) | ⬜ Planned | Millions of quotes, needs wikitext parser |
| [GoodReads](https://goodreads.com/quotes) | Crawler | ⬜ Planned | Millions of quotes, aggressive bot detection |
| [Kaggle Dataset](https://www.kaggle.com/datasets/akmittal/quotes-dataset) | CSV Import | ⬜ Planned | 500k+ quotes, bulk seed |

## Stack

- **Language:** Go
- **Database:** PostgreSQL
- **Migrations:** Goose
- **HTML Parsing:** goquery
- **Queue:** Redis + Asynq (planned)

## Architecture

```
cmd/
├── crawler/        → crawler binary
internal/
├── crawler/        → crawler.Run() orchestration (planned)
├── fetcher/        → HTTP logic + rate limiting
├── parser/         → site-specific parsers (interface + implementations)
├── dedup/          → normalization, SHA256, simhash, hamming distance
└── db/             → postgres connection, migrations, storage
```

## Deduplication

Two-layer dedup system to prevent both exact and near-duplicate quotes from entering the DB.

```
new quote
    │
    ├─ normalize + strip quote chars (dedup.Normalize, dedup.StripQuoteChars)
    │
    ├─ SHA256 match? → exact duplicate → discard        ✅ implemented
    │   (ON CONFLICT DO NOTHING in SaveQuote)
    │
    └─ Hamming distance < threshold? → near duplicate → discard   ✅ implemented
        (LSH banding in Redis, Hamming check on candidates only)
```

### What's built

| Function | Status | Notes |
|---|---|---|
| `dedup.Normalize` | ✅ Done | Lowercase, strip punctuation, whitespace |
| `dedup.StripQuoteChars` | ✅ Done | Strips `"` `"` `"` `«` `»` before saving |
| `dedup.SHA256` | ✅ Done | Exact duplicate fingerprint |
| `dedup.Simhash` | ✅ Done | Near-duplicate fingerprint |
| `dedup.HammingDistance` | ✅ Done | Bit distance between two simhashes |
| `dedup.ExtractBands` | ✅ Done | LSH banding — splits simhash into 4 × 16-bit bands |
| Exact dedup in `SaveQuote` | ✅ Done | `ON CONFLICT (sha256_hash) DO NOTHING` |
| Near-dedup via Redis LSH | ✅ Done | Band lookup → candidate set → Hamming check |
| `WarmSimhashCache` | ✅ Done | Loads all simhashes from Postgres into Redis on startup |

### Redis simhash cache

```
on startup → WarmSimhashCache: load all simhashes from Postgres → Redis Sets (LSH bands)
on insert  → check Hamming distance against Redis candidates only (~0.1ms vs ~20ms Postgres)
on save    → write to Postgres + add simhash bands to Redis

Redis restart → always re-warm from Postgres (source of truth)
```

Memory cost: ~5-10MB for 100k quotes (simhash = int64 = 8 bytes per quote).

### LSH Banding

Instead of checking every stored simhash, the 64-bit simhash is split into 4 bands of 16 bits each. Similar quotes will share at least one band, so only candidates from matching buckets are Hamming-checked.

```
simhash (64 bits) → band0 | band1 | band2 | band3  (16 bits each)
each band → Redis key: simhash:band:<n>:<value>
new quote → lookup 4 keys → collect candidates → Hamming check only on candidates
```

At 10M quotes: ~152 Hamming checks per insert instead of 10M.

## Tests

| Package | Coverage | Notes |
|---|---|---|
| `internal/dedup` | 96.6% | All core functions covered |
| `internal/parser` | 91.7% | Parser tested against fixture HTML |
| `internal/db` | 64.2% | SaveQuote tested with testcontainers (Postgres + Redis) |

Tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to spin up isolated Postgres and Redis containers — no manual setup needed.

```bash
go test ./...               # run all tests
go test -v ./...            # verbose output
go test -cover ./...        # with coverage
```

## URL Frontier & Priority Queue

Instead of hardcoded page loops, the crawler will maintain a **URL frontier** — a priority queue of URLs waiting to be crawled. Pages are discovered dynamically during parsing and re-enqueued with a score.

```
Seed URLs → [Redis Sorted Set] → Worker pulls lowest score URL → Fetch → Parse
                 ↑                                                          |
                 └──────────── new URLs enqueued with score ───────────────┘
                                          +
                                    quotes → DB
```

### Scoring

Lower score = crawled sooner:

```
score = source_base + (depth × depth_penalty) + (errors × error_penalty)
```

| Factor | Effect |
|---|---|
| Source priority | Base weight per domain (Quotable = 1, BrainyQuote = 5, Goodreads = 20) |
| Crawl depth | Penalty per page level deeper |
| Error rate | Penalty for past failures on this domain |

### Redis primitives used

| Structure | Purpose |
|---|---|
| `Sorted Set` — `frontier` | Priority queue (`ZADD` to push, `ZPOPMIN` to pull) |
| `Set` — `visited` | Dedup visited URLs (`SADD`, `SISMEMBER`) |
| `Set` — `simhash:band:*` | Near-duplicate quote detection via LSH |

### Asynq weighted queues

```
critical (weight 6) → high-yield, reliable sources (Quotable API)
default  (weight 3) → mid-tier sources (BrainyQuote)
low      (weight 1) → slow or unreliable sources (Goodreads)
```

## TODO

### Phase 1 — Foundations
- ✅ Move crawler loop from `main.go` → `internal/crawler/crawler.go`
- ✅ Add rate limiting to `fetcher` (configurable delay per domain)
- ✅ Write tests for `dedup`, `parser`, `db`
- [ ] Replace hardcoded page loop with dynamic next-page detection

### Phase 2 — New Sources
- [ ] Quotable API fetcher + parser (`internal/parser/quotable.go`)
- [ ] BrainyQuote parser (`internal/parser/brainyquote.go`)
- [ ] Wikiquote parser via MediaWiki API (`internal/parser/wikiquote.go`)
- [ ] CSV importer for Kaggle dataset

### Phase 3 — Queue & Infrastructure
- [ ] Redis URL frontier using Sorted Set (`ZADD` / `ZPOPMIN`)
- [ ] Visited URL set to prevent re-crawling
- [ ] URL scoring system (source base + depth + error rate)
- [ ] Asynq workers with weighted queues (critical / default / low)

### Phase 4 — Robustness
- [ ] Per-domain error tracking + automatic backoff
- [ ] Retry logic in fetcher
- [ ] Graceful shutdown (context cancellation)
- [ ] Structured logging (slog or zap)
- [ ] Metrics (crawl rate, save rate, error rate)

