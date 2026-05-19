# Source Asia Backend Assignment (Go)

Runnable **Go** HTTP service covering **Part 1** (rate-limited API) and **Part 2** (product catalog). Both parts share one process on port **8080**.

## Requirements

- Go **1.22+**

## Run

```bash
go run ./cmd/server
# or
make run
```

Base URL: `http://localhost:8080`

## Verify (manual)

With the server running in another terminal:

```bash
./scripts/e2e-smoke.sh
```

The script checks Part 1 (rate limit, stats, validation) and Part 2 (products CRUD, list shape, media append).

### AI tools

AI-assisted tooling (Cursor) was used for scaffolding and README.

---

## Part 1 — Rate-limited API

### Design

| Topic | Implementation |
|-------|----------------|
| Window | **Rolling** 60 seconds (`WINDOW_MILLIS`) |
| Limit | **5** accepts per `user_id` (`MAX_REQUESTS_PER_MINUTE`) |
| Storage | In-memory `map[user_id]*userState` |
| Concurrency | **Per-user `sync.Mutex`** on timestamp deque + counters; map guarded separately |
| Over limit | **429** + JSON `message` + `Retry-After` header |
| Success | **200 OK** (assignment allows 200 or 201) |
| `total_rejected` | **Cumulative** lifetime **429** count (documented) |

### `POST /request`

**Request**

```json
{
  "user_id": "alice",
  "payload": { "any": "json value" }
}
```

**200**

```json
{
  "status": "accepted",
  "user_id": "alice",
  "requests_in_window": 3
}
```

**429**

```json
{
  "timestamp": "2026-05-19T12:00:00Z",
  "status": 429,
  "error": "Too Many Requests",
  "message": "Rate limit exceeded for user: alice",
  "path": "/request"
}
```

**400** — missing/blank `user_id`, missing/`null` `payload`, invalid JSON.

```bash
curl -s -X POST http://localhost:8080/request \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"alice","payload":{"job":"send-email"}}'
```

### `GET /stats`

```bash
curl -s http://localhost:8080/stats
```

```json
{
  "users": [
    {
      "user_id": "alice",
      "total_accepted": 12,
      "total_rejected": 3,
      "requests_in_current_window": 5
    }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `requests_in_current_window` | Timestamps still inside the rolling window |
| `total_rejected` | Lifetime **429** responses for that user |
| `total_accepted` | Lifetime successful admissions |

---

## Part 2 — Product catalog

### API

| Method | Path | Success |
|--------|------|---------|
| `POST` | `/products` | **201** + full product |
| `GET` | `/products?limit=&offset=` | **200** paginated summaries |
| `GET` | `/products/{id}` | **200** full media arrays |
| `POST` | `/products/{id}/media` | **200** append URLs |

### List vs detail (performance)

```
Store
  byID  map[UUID]*Product   // holds image_urls[] and video_urls[]
  bySKU map[string]UUID
```

- **Detail / create response:** returns full `image_urls` and `video_urls`.
- **List (`GET /products`):** paginates first (`offset`, `limit`), builds only `image_count`, `video_count`, `thumbnail_url` (first image) per row — **does not** serialize every stored URL.

With 1,000 products × 10 images, `GET /products?limit=20` touches **20** rows for the response body, not 10,000 URL strings.

### Example

```bash
curl -s -X POST http://localhost:8080/products \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Widget A",
    "sku": "SKU-001",
    "image_urls": ["https://cdn.example.com/sku-001/1.jpg"],
    "video_urls": ["https://cdn.example.com/sku-001/demo.mp4"]
  }'
```

### Validation

| Rule | Value |
|------|-------|
| `name`, `sku` | Required, non-empty |
| URL scheme | `http://` or `https://` |
| Max URL length | **2048** |
| Max URLs per array per request | **20** |
| Duplicate `sku` | **409 Conflict** |
| No file upload / base64 | URL strings only |

### Pagination

| Param | Default | Max |
|-------|---------|-----|
| `limit` | `20` | `100` |
| `offset` | `0` | — |

---

## Configuration

| Variable | Default |
|----------|---------|
| `ADDR` | `:8080` |
| `MAX_REQUESTS_PER_MINUTE` | `5` |
| `WINDOW_MILLIS` | `60000` |
| `MAX_URLS_PER_REQUEST` | `20` |
| `MAX_URL_LENGTH` | `2048` |
| `DEFAULT_LIMIT` | `20` |
| `MAX_LIMIT` | `100` |

---

## Production limitations

- **Single process** — state is in RAM; restart clears rate limits and products.
- **Not horizontally scalable** without shared storage (Redis for rate limits; PostgreSQL for catalog).
- **No authentication** — `user_id` is trusted from the client.
- **Stats** — `GET /stats` scans all users; would need pagination or metrics export at scale.
- **`Retry-After`** — coarse seconds value based on window length, not exact time-until-slot-frees.

### With PostgreSQL + CDN (future)

- `products` + `product_media` tables; counts via SQL `COUNT(*)`.
- CDN serves assets; API stores URLs only.
- Redis sorted sets (or similar) for distributed sliding-window rate limits.

---

## Project structure

```
.
├── cmd/server/              # Application entrypoint
├── internal/
│   ├── config/              # Configuration from environment
│   ├── server/              # HTTP router (wires handlers)
│   ├── httpx/               # Shared JSON HTTP helpers
│   ├── id/                  # UUID utilities
│   ├── ratelimit/           # Part 1: limiter (domain) + handler (HTTP)
│   └── product/             # Part 2: model, store, service, handler
├── scripts/e2e-smoke.sh     # Manual smoke checks (optional)
├── docs/ARCHITECTURE.md     # Layering and data flow
├── Makefile
└── go.mod
```

Each package follows a clear split:

| Layer | Responsibility |
|-------|----------------|
| **handler** | HTTP parsing, status codes, JSON envelopes |
| **service** | Validation and business rules (product) |
| **store / limiter** | In-memory state, concurrency |
| **model / types** | Domain and API shapes |

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for diagrams and concurrency notes.

## Submission checklist

| Requirement | Status |
|-------------|--------|
| Part 1 endpoints + rate limit + concurrency | Done |
| Part 2 CRUD + media + list without full URLs | Done |
| README (run, curl, schemas, limitations) | Done |
| Go (preferred language) | Done |
| Manual smoke script (`scripts/e2e-smoke.sh`) | Included |
