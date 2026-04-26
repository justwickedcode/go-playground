# quotes-crawler

A scalable web crawler that collects quotes from multiple sources and stores them in PostgreSQL.

## Sources

| Source | Method | Status | Notes |
|---|---|---|---|
| [quotes.toscrape.com](https://quotes.toscrape.com) | Crawler | ✅ Done | Sandbox site, 100 quotes |
| [BrainyQuote](https://brainyquote.com) | Crawler | ⬜ Planned | 100k+ quotes, clean HTML |
| [Wikiquote](https://en.wikiquote.org) | Crawler (MediaWiki API) | ⬜ Planned | Millions of quotes, needs wikitext parser |
| [GoodReads](https://goodreads.com/quotes) | Crawler | ⬜ Planned | Millions of quotes, aggressive bot detection |
| [Quotable API](https://api.quotable.io) | API Import | ⬜ Planned | 5000+ curated quotes, no auth needed |
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
├── fetcher/        → HTTP logic
├── parser/         → site-specific parsers (interface + implementations)
├── dedup/          → normalization, SHA256, simhash, hamming distance
└── db/             → postgres connection, migrations, storage
```
