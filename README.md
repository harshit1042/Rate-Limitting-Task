# Source Asia Backend Assignment (Go)

Runnable **Go** HTTP service: **Part 1** (rate-limited API) and **Part 2** (product catalog) in one process on port **8080**.

## Requirements

- Go **1.22+** (uses `http.ServeMux` method routing and `PathValue`)

## Quick start

```bash
go run ./cmd/server
# or
make run
```

Base URL: `http://localhost:8080`

Build a binary:

```bash
make build    # → bin/server
```

## Verify

In a second terminal (server must be running):

```bash
./scripts/e2e-smoke.sh
```

The script checks Part 1 (rate limit, stats, validation) and Part 2 (create, list shape, detail, media append, error codes). Last run: **24 assertions, all passing**.

For layering, concurrency, and data flow, see **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

---

## API overview

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/request` | Submit a rate-limited request |
| `GET` | `/stats` | Per-user rate limit counters |
| `POST` | `/products` | Create product |
| `GET` | `/products` | Paginated list (summaries only) |
| `GET` | `/products/{id}` | Product detail (full media URLs) |
| `POST` | `/products/{id}/media` | Append image/video URLs |

### Error format

Errors use a shared JSON envelope (`internal/httpx`):

```json
{
  "timestamp": "2026-05-19T12:00:00Z",
  "status": 400,
  "error": "Bad Request",
  "message": "user_id must not be blank",
  "path": "/request"
}
```

---

## Part 1 — Rate-limited API

### Design

| Topic | Implementation |
|-------|----------------|
| Window | **Rolling** 60 seconds (`WINDOW_MILLIS`) |
| Limit | **5** accepts per `user_id` (`MAX_REQUESTS_PER_MINUTE`) |
| Storage | In-memory `map[user_id]*userState` |
| Concurrency | Map `sync.Mutex` + per-user `sync.Mutex` on timestamp deque |
| Over limit | **429** + JSON `message` + `Retry-After` header |
| Success | **200 OK** |
| `total_rejected` | **Cumulative** lifetime **429** count per user |

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

**400** — blank/missing `user_id`, missing/`null` `payload`, invalid JSON.

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
| `requests_in_current_window` | Admissions still inside the rolling window |
| `total_rejected` | Lifetime **429** responses for that user |
| `total_accepted` | Lifetime successful admissions |

Users are sorted by `user_id`.

---

## Part 2 — Product catalog

### Endpoints

| Method | Path | Success | Notes |
|--------|------|---------|-------|
| `POST` | `/products` | **201** | Full product body |
| `GET` | `/products?limit=&offset=` | **200** | Summaries: `image_count`, `video_count`, `thumbnail_url` |
| `GET` | `/products/{id}` | **200** | Full `image_urls` / `video_urls` |
| `POST` | `/products/{id}/media` | **200** | Append URLs; returns updated product |

**409** — duplicate `sku`. **404** — unknown product. **400** — validation (URLs, empty media body, etc.).

### Create product

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

**201** example (abbreviated):

```json
{
  "id": "f563965a-6db1-4f32-8c83-12d6894c1e4f",
  "name": "Widget A",
  "sku": "SKU-001",
  "image_urls": ["https://cdn.example.com/sku-001/1.jpg"],
  "video_urls": ["https://cdn.example.com/sku-001/demo.mp4"],
  "created_at": "2026-05-19T12:00:00Z"
}
```

### List products

```bash
curl -s 'http://localhost:8080/products?limit=20&offset=0'
```

```json
{
  "items": [
    {
      "id": "f563965a-6db1-4f32-8c83-12d6894c1e4f",
      "name": "Widget A",
      "sku": "SKU-001",
      "image_count": 1,
      "video_count": 1,
      "thumbnail_url": "https://cdn.example.com/sku-001/1.jpg",
      "created_at": "2026-05-19T12:00:00Z"
    }
  ],
  "limit": 20,
  "offset": 0,
  "total_count": 1
}
```

List responses **do not** include full `image_urls` / `video_urls` arrays (performance). Use `GET /products/{id}` for full media.

### Append media

```bash
curl -s -X POST http://localhost:8080/products/{id}/media \
  -H 'Content-Type: application/json' \
  -d '{"image_urls":["https://cdn.example.com/sku-001/2.jpg"]}'
```

At least one of `image_urls` or `video_urls` must be non-empty.

### Validation

| Rule | Value |
|------|-------|
| `name`, `sku` | Required, non-empty (trimmed) |
| URL scheme | `http://` or `https://` |
| Max URL length | **2048** (`MAX_URL_LENGTH`) |
| Max URLs per array per request | **20** (`MAX_URLS_PER_REQUEST`) |
| Duplicate `sku` | **409 Conflict** |
| Media | URL strings only — no file upload or base64 |

### Pagination

| Param | Default | Max |
|-------|---------|-----|
| `limit` | `20` | `100` |
| `offset` | `0` | — |

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | Listen address |
| `MAX_REQUESTS_PER_MINUTE` | `5` | Rate limit per user per window |
| `WINDOW_MILLIS` | `60000` | Rolling window length (ms) |
| `MAX_URLS_PER_REQUEST` | `20` | Max URLs per array in one request |
| `MAX_URL_LENGTH` | `2048` | Max characters per URL |
| `DEFAULT_LIMIT` | `20` | Default page size for `GET /products` |
| `MAX_LIMIT` | `100` | Maximum `limit` query param |

---

## Project structure

```
.
├── cmd/server/              # Entrypoint, graceful shutdown
├── internal/
│   ├── config/              # Environment configuration
│   ├── server/              # HTTP router (wires handlers)
│   ├── httpx/               # JSON encode/decode helpers
│   ├── id/                  # UUID utilities
│   ├── ratelimit/           # Part 1: limiter + HTTP handler
│   └── product/             # Part 2: model, store, service, handler
├── scripts/e2e-smoke.sh     # End-to-end smoke tests
├── docs/ARCHITECTURE.md     # Design, flows, concurrency
├── Makefile
└── go.mod
```

| Layer | Responsibility |
|-------|----------------|
| **handler** | HTTP parsing, status codes, JSON |
| **service** | Business rules and validation (product) |
| **limiter / store** | In-memory state and locking |
| **model / types** | Domain and API shapes |

---

## Production limitations

- **Single process** — state is in RAM; restart clears rate limits and products.
- **Not horizontally scalable** without shared storage (e.g. Redis for limits, PostgreSQL for catalog).
- **No authentication** — `user_id` is taken from the client as-is.
- **`GET /stats`** — scans all users; needs pagination or metrics export at scale.
- **`Retry-After`** — seconds equal to full window length, not exact time until a slot frees.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for a scaling roadmap and detailed concurrency notes.

---

## Submission checklist

| Requirement | Status |
|-------------|--------|
| Part 1 endpoints + rate limit + concurrency | Done |
| Part 2 CRUD + media + list without full URLs | Done |
| README (run, curl, schemas, limitations) | Done |
| Architecture documentation | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Go (preferred language) | Done |
| E2E smoke script (`scripts/e2e-smoke.sh`) | Included |
