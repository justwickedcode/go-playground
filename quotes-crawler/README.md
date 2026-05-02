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
    └─ Hamming distance < threshold? → near duplicate → discard   ⬜ pending Redis
        (check against Redis simhash cache)
```

### What's built

| Function | Status | Notes |
|---|---|---|
| `dedup.Normalize` | ✅ Done | Lowercase, strip punctuation, whitespace |
| `dedup.StripQuoteChars` | ✅ Done | Strips `"` `"` `"` `«` `»` before saving |
| `dedup.SHA256` | ✅ Done | Exact duplicate fingerprint |
| `dedup.Simhash` | ✅ Done | Near-duplicate fingerprint |
| `dedup.HammingDistance` | ✅ Done | Bit distance between two simhashes |
| Exact dedup in `SaveQuote` | ✅ Done | `ON CONFLICT (sha256_hash) DO NOTHING` |
| Near-dedup via Redis | ⬜ Pending | TODO in `SaveQuote`, blocked on Redis |

### Redis simhash cache (planned)

```
on startup → WarmSimhashCache: load all simhashes from Postgres → Redis SET
on insert  → check Hamming distance against Redis in memory (~0.1ms vs ~20ms Postgres)
on save    → write to Postgres + add simhash to Redis SET

Redis restart → always re-warm from Postgres (source of truth)
```

Memory cost: ~5-10MB for 100k quotes (simhash = int64 = 8 bytes per quote).

## URL Frontier & Priority Queue

Instead of hardcoded page loops, the crawler maintains a **URL frontier** — a priority queue of URLs waiting to be crawled. Pages are discovered dynamically during parsing and re-enqueued with a score.

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
| `Set` — `simhash_cache` | Near-duplicate quote detection |

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
- [ ] Redis simhash cache + `WarmSimhashCache` on startup
- [ ] Near-dedup via Hamming distance check against Redis cache

### Phase 4 — Robustness
- [ ] Per-domain error tracking + automatic backoff
- [ ] Retry logic in fetcher
- [ ] Graceful shutdown (context cancellation)
- [ ] Structured logging (slog or zap)
- [ ] Metrics (crawl rate, save rate, error rate)
- 