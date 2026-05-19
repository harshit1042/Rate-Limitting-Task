# Architecture

## Layers

```
cmd/server/          Entrypoint: config load, HTTP server lifecycle
internal/
  config/            Environment-based settings
  server/            HTTP route table
  httpx/             Shared JSON request/response helpers
  id/                UUID generation and parsing
  ratelimit/         Part 1
    limiter.go       Domain: sliding-window rate limit (concurrency-safe)
    types.go         API DTOs
    handler.go       HTTP adapters
  product/           Part 2
    model.go         Domain models + input types
    errors.go        Domain errors
    store.go         In-memory repository
    service.go       Business rules + validation orchestration
    validate.go      URL validation rules
    handler.go       HTTP adapters (thin)
```

## Request flow

**Part 1**

```
POST /request → ratelimit.Handler → Limiter.TryAccept → 200 | 429
GET  /stats   → ratelimit.Handler → Limiter.Snapshot
```

**Part 2**

```
POST /products           → product.Handler → Service.Create → Store
GET  /products           → product.Handler → Service.List   → Store (summary rows only)
GET  /products/{id}      → product.Handler → Service.Get    → Store
POST /products/{id}/media → product.Handler → Service.AppendMedia → Store
```

## Concurrency (Part 1)

- One `sync.Mutex` protects the user map.
- Each user has a dedicated `sync.Mutex` for its timestamp deque and counters.
- `TryAccept` prune + check + append run in one critical section per user.

## List vs detail (Part 2)

- **Store** holds full `image_urls` / `video_urls` slices per product.
- **ListPage** paginates first, then builds `ListItem` with counts + first image only.
- **GetByID** returns a defensive copy with all URLs for the detail view.
