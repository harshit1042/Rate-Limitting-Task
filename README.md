# Source Asia Backend Assignment (Go)

Runnable **Go** HTTP service: **Part 1** (rate-limited API) and **Part 2** (product catalog) in one process on port **8080**.

## Setup

### Prerequisites

- **Go 1.22+** (method-based `http.ServeMux` routes and `PathValue` for product ids)
- No database, Redis, or Docker required for local run

### Run the server

From the repository root:

```bash
go run ./cmd/server
```

Or:

```bash
make run
```

The server listens on **`http://localhost:8080`** by default (`ADDR=:8080`).

Build a standalone binary:

```bash
make build
./bin/server
```

### Verify

With the server running, open a second terminal:

```bash
chmod +x scripts/e2e-smoke.sh   # once, if needed
./scripts/e2e-smoke.sh
```

Optional: point at another host/port:

```bash
BASE_URL=http://127.0.0.1:8080 ./scripts/e2e-smoke.sh
```

The script exercises rate limiting, stats, product CRUD, list-vs-detail shape, media append, and common error responses.

### Configuration (environment)

| Variable | Default | Description |
|----------|---------|-------------|
| `ADDR` | `:8080` | Listen address |
| `MAX_REQUESTS_PER_MINUTE` | `5` | Rate limit per user per window |
| `WINDOW_MILLIS` | `60000` | Rolling window (ms) |
| `MAX_URLS_PER_REQUEST` | `20` | Max URLs per array per request |
| `MAX_URL_LENGTH` | `2048` | Max characters per URL |
| `DEFAULT_LIMIT` | `20` | Default `GET /products` page size |
| `MAX_LIMIT` | `100` | Max `limit` query param |

Example:

```bash
export MAX_REQUESTS_PER_MINUTE=10
export WINDOW_MILLIS=30000
go run ./cmd/server
```

Further design detail: **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

---

## Assumptions

### General

- **Single process** — one binary serves Part 1 and Part 2; all state is in memory.
- **No authentication** — `user_id` (Part 1) and all catalog endpoints are open; callers are trusted for identity.
- **JSON only** — `Content-Type: application/json` on request bodies where a body is required.
- **Graceful shutdown** — `SIGINT` / `SIGTERM` triggers a bounded HTTP shutdown (10s); in-flight work is not drained beyond that.

### Part 1 — rate limiting

- **Rolling window** — not fixed clock-minute buckets; old admissions fall out after `WINDOW_MILLIS`.
- **`user_id` is opaque** — any non-blank string after trim; no format validation.
- **`payload` is validated but not stored** — must be present, non-null JSON; the limiter only counts admissions.
- **Success status is 200** — assignment allows 200 or 201; this implementation uses **200** for accepted requests.
- **`total_rejected` is lifetime** — cumulative 429 count per user, not “rejects in current window”.
- **`Retry-After`** — set to the full window length in seconds when rate limited, not the exact time until the next slot opens.
- **Per-user serialization** — one mutex per user for timestamp deque updates; map of users guarded separately.

### Part 2 — product catalog

- **Media is URL-only** — `http://` or `https://` strings; no multipart upload, base64, or binary storage in the API.
- **Product ids are UUID v4** — invalid id format → **400**; valid id but missing row → **404**.
- **SKU is unique** — duplicate on create → **409**; SKU is not updatable via a separate endpoint.
- **Empty media on create is allowed** — `image_urls` / `video_urls` may be omitted or `[]`; URL rules apply only when arrays are non-empty.
- **Append media** — at least one non-empty `image_urls` or `video_urls` array required on `POST /products/{id}/media`.
- **List vs detail** — list returns summaries (`image_count`, `video_count`, optional `thumbnail_url`); detail returns full URL arrays.
- **Pagination** — `offset` / `limit` with defaults; list sorted by `created_at` then `id`.

---

## Tradeoffs

| Area | Choice | Why | Cost |
|------|--------|-----|------|
| **Persistence** | In-memory maps | Simple, fast for assignment scope | Data lost on restart; not multi-instance |
| **Rate limit store** | Per-user mutex + timestamp slice | Correct sliding window under concurrency without one global lock | Memory grows with distinct `user_id`s; `GET /stats` scans all users |
| **Catalog store** | `byID` + `bySKU` maps, URL slices on `Product` | O(1) lookup, straightforward cloning | List sorts entire catalog in memory before slicing a page |
| **List response** | `ListItem` without full URL arrays | Meets performance requirement (e.g. 1k×10 images → ≤20 thumbnails in JSON, not 10k URLs) | Clients need a second request for full media |
| **List pagination** | Sort all products, then `[offset:limit]` | Easy to implement in one process | O(n) per list request on catalog size, not O(page size) |
| **Error shape** | Shared JSON envelope + `path` | Consistent client handling | Slightly more verbose than plain text errors |
| **Part 1 layout** | Handler + limiter in one package | Small domain; fewer files | Less symmetry with Part 2’s handler → service → store |
| **Part 2 layout** | Handler → service → store | Clear validation and HTTP separation | More layers for a small feature set |
| **Config** | Environment variables with defaults | No config file dependency | Invalid env ints silently fall back to defaults |
| **Production path** | Documented PostgreSQL + CDN + Redis | Keeps assignment scope small while showing scale path | Not implemented in this repo |

### Known limitations (current build)

- Not horizontally scalable without **Redis** (rate limits) and **PostgreSQL** (catalog).
- **`GET /stats`** returns every user with no pagination.
- **No rate limit on catalog endpoints** — only `POST /request` is throttled.
- **No delete/update product** — create, read, list, append media only.

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

### Data model (in-memory)

Products and media live in a single in-memory store (`internal/product/store.go`). There is no separate “media table” — images and videos are URL string slices on each product.

```
Store
  mu    sync.RWMutex
  byID  map[UUID]*Product      // primary index: id → product
  bySKU map[string]UUID        // secondary index: sku → id (duplicate check)

Product (one row per catalog item)
  id, name, sku, created_at
  image_urls []string          // e.g. https://cdn.example.com/.../1.jpg
  video_urls []string          // e.g. https://cdn.example.com/.../demo.mp4
```

| Aspect | How it works |
|--------|----------------|
| **Storage** | Each `Product` holds `image_urls` and `video_urls` as `[]string` in RAM |
| **Lookup by id** | `byID[id]` → full product |
| **Lookup by sku** | `bySKU[sku]` → id, then `byID` (create rejects duplicate SKU with **409**) |
| **Concurrency** | `sync.RWMutex`: shared lock for reads, exclusive lock for writes |
| **Safety** | `GetByID`, `Create`, and `AppendMedia` return **clones** so callers cannot mutate internal maps |
| **No uploads** | API accepts URL strings only — no binary files or base64 in the request body |

### List vs detail (how queries differ)

The API uses two response shapes on purpose: a **summary** for grids and a **full product** when you open one item.

| | `GET /products` (list) | `GET /products/{id}` (detail) |
|--|------------------------|-------------------------------|
| **JSON type** | `ListItem` inside `items[]` | `Product` |
| **`image_urls` / `video_urls`** | **Not included** | Full arrays |
| **`image_count` / `video_count`** | Yes | Use array lengths on detail |
| **`thumbnail_url`** | First image URL only (if any) | N/A |
| **Store path** | `ListPage` → `toListItem` per row on the page | `GetByID` → `cloneProduct` |
| **Typical use** | Product grid, search results | Product page, edit form |

**List flow (`GET /products?limit=20&offset=0`):**

1. Parse `limit` / `offset` (defaults 20 / 0, max limit 100).
2. Sort products by `created_at`, then `id`.
3. Take only the page slice (`offset` … `offset+limit`).
4. For each row on that page, build a `ListItem`: `len(image_urls)`, `len(video_urls)`, and optionally `thumbnail_url` = first image.
5. Serialize **only** those summary fields — never the full URL lists.

**Detail flow (`GET /products/{id}`):**

1. Load one product from `byID`.
2. Return a defensive copy with **all** `image_urls` and `video_urls`.

Create (`POST /products`) and append media (`POST /products/{id}/media`) return the **detail** shape (full arrays), same as `GET /products/{id}`.

### List performance (1,000 products × 10 images)

Suppose the catalog has **1,000 products** and each has **10 image URLs** stored (10,000 URL strings in memory total).

`GET /products?limit=20` must **not** load or serialize all 10,000 image URLs in the HTTP response. This service satisfies that:

| What happens | Count for this example |
|--------------|-------------------------|
| URLs stored in RAM (normal catalog size) | 10,000 (across all products) |
| Products included in JSON `items` | 20 |
| Full `image_urls` arrays in list JSON | **0** |
| `thumbnail_url` strings in list JSON | **≤ 20** (one per list row, first image only) |
| URL strings in list JSON (thumbnails only) | **≤ 20**, not 10,000 |

So the list endpoint returns only what a grid needs: metadata, counts, and at most one preview URL per visible row. Clients that need every image call `GET /products/{id}` for that product.

**In-memory caveat:** The current store sorts all product pointers to paginate. That touches every product record for ordering but still builds and serializes **only the current page** of summaries. With PostgreSQL (below), sorting and pagination move into the database so the app does not need to walk the full catalog in memory.

### Production: PostgreSQL + CDN

Today everything is in one process and one machine’s RAM. For production you would keep the **same API contract** (list summaries vs detail URLs) but change persistence and asset delivery:

**CDN (assets)**

- Images and videos are served by a **CDN** (CloudFront, Cloudflare, etc.).
- The API stores and returns **URLs only** — same as now. No file bytes through the catalog API.
- Upload flows (out of scope for this assignment) would write to object storage (S3) and register the CDN URL in the catalog.

**PostgreSQL (catalog)**

| Table | Purpose |
|-------|---------|
| `products` | `id`, `name`, `sku` (unique), `created_at` |
| `product_media` | `product_id`, `type` (`image` \| `video`), `url`, `sort_order` |

**List query (grid)** — do not `SELECT url` for every row:

```sql
SELECT p.id, p.name, p.sku, p.created_at,
       COUNT(*) FILTER (WHERE m.type = 'image') AS image_count,
       COUNT(*) FILTER (WHERE m.type = 'video') AS video_count,
       (SELECT url FROM product_media
        WHERE product_id = p.id AND type = 'image'
        ORDER BY sort_order LIMIT 1) AS thumbnail_url
FROM products p
LEFT JOIN product_media m ON m.product_id = p.id
GROUP BY p.id
ORDER BY p.created_at, p.id
LIMIT $1 OFFSET $2;
```

**Detail query** — one product, all URLs:

```sql
SELECT url FROM product_media
WHERE product_id = $1 AND type = 'image'
ORDER BY sort_order;
-- same for video
```

| Concern | In-memory (this repo) | PostgreSQL + CDN |
|---------|----------------------|------------------|
| Durability | Lost on restart | Durable |
| Horizontal scale | Single instance | Multiple API instances, shared DB |
| List payload | `ListItem` summaries | Same JSON shape; SQL aggregates counts |
| Media bytes | Not stored in API | CDN serves files; DB holds URLs |
| Hot lists | N/A | Optional Redis cache for popular `limit`/`offset` pages |

Rate limits (Part 1) would move to **Redis** (e.g. sorted-set sliding window) so all instances share per-user counters. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for handler layering and concurrency diagrams.

---

## Project structure

```
.
├── cmd/server/
├── internal/
│   ├── config/
│   ├── server/
│   ├── httpx/
│   ├── id/
│   ├── ratelimit/
│   └── product/
├── scripts/e2e-smoke.sh
├── docs/ARCHITECTURE.md
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

## Submission checklist

| Requirement | Status |
|-------------|--------|
| Part 1 endpoints + rate limit + concurrency | Done |
| Part 2 CRUD + media + list without full URLs | Done |
| README (run, curl, schemas, limitations) | Done |
| Architecture documentation | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Go (preferred language) | Done |
| E2E smoke script (`scripts/e2e-smoke.sh`) | Included |
