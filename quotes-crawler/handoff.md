# Handoff — quotes-crawler

## What we did this session

### Migrations
- Switched fully to Goose for all migrations (dropped the old golang-migrate style file)
- Migration 1: `create_quotes` — quotes table, unchanged
- Migration 2: `create_url_frontier` — new table with `crawl_status` ENUM (`pending`, `in_progress`, `done`, `failed`), composite index on `(status, priority)`

### New package: `internal/scoring`
- `scoring.go` — `CalculatePriority(source, depth, errorCount)`
- Exported constants: `DepthPenalty = 0.5`, `ErrorPenalty = 3.0`, `DefaultSourceBase = 1000.0`
- Source base scores: Quotable = 1.0, BrainyQuote = 5.0, Goodreads = 20.0
- Unknown sources fall back to `DefaultSourceBase` (crawled last, no crash)

### New model: `models.URLFrontier`
- Fields match `url_frontier` table exactly
- `LastCrawledAt` is `*time.Time` (nullable)
- Naming follows Go conventions: `URL` not `Url`, `URLFrontier` not `UrlFrontier`

### New storage functions in `internal/db`
All methods on `*Store`, same pattern as `SaveQuote`:

| Function | What it does |
|---|---|
| `SaveURL(ctx, URLFrontier)` | Insert URL, `ON CONFLICT DO NOTHING`, returns `(bool, error)` |
| `MarkURLDone(ctx, url)` | Sets `status=done`, `last_crawled_at=NOW()` |
| `MarkURLFailed(ctx, url)` | Sets `status=failed`, `last_crawled_at=NOW()`, `error_count+1` |
| `GetPendingURLs(ctx)` | Returns all `status=pending` rows ordered by priority |
| `WarmFrontierCache(ctx)` | Loads pending URLs into Redis Sorted Set on startup |
| `PushURL(ctx, url, priority)` | `ZADD frontier <priority> <url>` |
| `PopURL(ctx)` | `ZPOPMIN frontier` — returns next URL to crawl |

---

## What's next (pick up here)

### 1. Seed URLs — immediate next step
On first run, the frontier is empty. Need a `SeedFrontier` function in `crawler.go` that:
- Defines a hardcoded list of seed URLs per source (BrainyQuote homepage, Quotable API root, etc.)
- Calls `SaveURL` for each one with `CalculatePriority(source, 0, 0)`
- Only seeds if the frontier is empty (check `GetPendingURLs` count first)

### 2. Wire frontier into `crawler.go`
Replace the current hardcoded loop in `crawler.Run()` with:
```
WarmSimhashCache → WarmFrontierCache → SeedFrontier (if empty) → pop loop
```
The pop loop: `PopURL` → fetch → parse → save quotes → push discovered URLs → mark done/failed

### 3. BrainyQuote parser (`internal/parser/brainyquote.go`)
First real source. Inspect `brainyquote.com/quotes` in the browser — look at:
- Quote container selector
- Author selector
- Next page link selector (this is where dynamic next-page detection actually matters)

### 4. Quotable API fetcher (`internal/parser/quotable.go`)
REST API, no HTML parsing needed. Endpoint: `https://api.quotable.io/quotes?page=1`
Returns JSON — much simpler than HTML crawling.

---

## Key decisions made

- **No Redis visited-URL set** — dedup handled by `UNIQUE` on `url_frontier.url` + `ON CONFLICT DO NOTHING`
- **Postgres ENUM for status** — `crawl_status` type, not plain TEXT strings. Adding new statuses requires a migration.
- **`scoring` is a separate package** — pure logic, no DB/Redis dependencies
- **toscrape.com skipped** — sandbox only, not a real source. Dynamic next-page detection will be built for BrainyQuote instead.

---

## Current migration state

```
Applied At                  Migration
=======================================
Sun May 24 17:22:41 2026 -- 20260524170104_create_quotes.sql
Sun May 24 17:47:11 2026 -- 20260524170105_create_url_frontier.sql
```

## Goose workflow reminder

```bash
goose create <name> sql     # create new migration
goose up                    # apply all pending
goose down                  # roll back one
goose status                # see what's applied
```

Env vars (set in your shell):
```
GOOSE_DRIVER=postgres
GOOSE_DBSTRING=postgres://user:password@localhost:5432/quotes?sslmode=disable
GOOSE_MIGRATION_DIR=./internal/db/migrations
```