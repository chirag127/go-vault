# go-vault

A production-grade URL shortener microservice written in Go. Live demo: `docker compose up` and it's running.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go)](go.mod)
[![CI](https://github.com/chirag127/go-vault/actions/workflows/ci.yml/badge.svg)](https://github.com/chirag127/go-vault/actions/workflows/ci.yml)

## Quick start

```bash
git clone https://github.com/chirag127/go-vault.git
cd go-vault
docker compose up
```

Endpoints:
- HTTP REST: `http://localhost:8080`
- gRPC: `localhost:9090`
- Prometheus metrics: `http://localhost:8080/metrics`
- Prometheus UI: `http://localhost:9091`

## Architecture

```mermaid
graph TD
    Client -->|HTTP REST| HTTPGateway[HTTP :8080]
    Client -->|gRPC| GRPCSrv[gRPC :9090]
    HTTPGateway --> Svc[ShortenerService]
    GRPCSrv --> Svc
    Svc -->|read-through| Cache[Redis]
    Svc -->|persist| PG[(PostgreSQL)]
    Svc -->|rate limit| Cache
    HTTPGateway --> Prom[/metrics]
    Prom --> Prometheus[Prometheus :9091]
```

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| gRPC | google.golang.org/grpc + hand-authored service stubs |
| Proto source | Protocol Buffers v3 (`api/proto/shortener.proto`) |
| HTTP gateway | chi v5 (thin REST layer over the same service) |
| Database | PostgreSQL 17 via `pgx/v5` |
| Cache + rate limit | Redis 7 via `go-redis/v9` |
| Migrations | Raw SQL (`migrations/`) — apply with `migrate/migrate` |
| Metrics | Prometheus (`client_golang`) |
| Logging | `log/slog` (structured JSON) |
| Config | `envconfig` (12-factor) |
| Container | Multi-stage Docker (distroless final image) |

## API — REST examples

```bash
# Create a short link
curl -X POST http://localhost:8080/v1/links \
  -H 'Content-Type: application/json' \
  -d '{"original_url":"https://example.com","ttl_seconds":86400}'

# Response: {"Code":"xYz9K2m","OriginalURL":"https://example.com",...}

# Redirect
curl -L http://localhost:8080/r/xYz9K2m

# Get link details
curl http://localhost:8080/v1/links/xYz9K2m

# Stats
curl http://localhost:8080/v1/links/xYz9K2m/stats

# List (paginated)
curl "http://localhost:8080/v1/links?page_size=10"

# Delete
curl -X DELETE http://localhost:8080/v1/links/xYz9K2m
```

## API — gRPC examples (grpcurl)

```bash
# Create a link
grpcurl -plaintext -d '{"original_url":"https://example.com"}' \
  localhost:9090 shortener.v1.ShortenerService/CreateLink

# Resolve
grpcurl -plaintext -d '{"code":"xYz9K2m","client_ip":"127.0.0.1"}' \
  localhost:9090 shortener.v1.ShortenerService/Resolve

# Stats
grpcurl -plaintext -d '{"code":"xYz9K2m"}' \
  localhost:9090 shortener.v1.ShortenerService/Stats
```

## Design highlights

### Caching (read-through, write-invalidate)
Redis is a read-through cache on `GetLink` and `Resolve` paths. On create, the new link is written to cache immediately. On click increment (fire-and-forget goroutine), the stale entry is evicted so next read picks up fresh click count from Postgres.

Cache TTL is the minimum of the global `CACHE_TTL` env var and the link's remaining lifetime — expired links are never served from cache.

### Rate limiting (Redis sliding counter)
Each inbound IP gets a Redis counter key (`rl:<ip>`). On each `Resolve` call:
1. Atomically `INCR` the key and set expiry in a pipeline.
2. If count > `RATE_LIMIT_MAX` within `RATE_LIMIT_WINDOW`, return `429`.

The rate limiter fails open on Redis errors so a cache outage doesn't block resolves.

### Short code generation (base62)
Codes are 7-char base62 (`[0-9A-Za-z]`) generated using `crypto/rand` for uniform distribution. Collision check runs up to 5 attempts before returning an error (collision probability at 62^7 ≈ 3.5 × 10¹² is negligible).

### Graceful shutdown
`signal.NotifyContext` captures `SIGINT`/`SIGTERM`. gRPC uses `GracefulStop()` (drains in-flight RPCs); HTTP uses `http.Server.Shutdown()` with a configurable timeout.

### Prometheus metrics
| Metric | Type | Labels |
|---|---|---|
| `govault_grpc_request_duration_seconds` | Histogram | `method`, `status` |
| `govault_http_request_duration_seconds` | Histogram | `method`, `path`, `status` |
| `govault_cache_hits_total` | Counter | — |
| `govault_cache_misses_total` | Counter | — |
| `govault_links_created_total` | Counter | — |
| `govault_links_resolved_total` | Counter | — |
| `govault_ratelimit_rejections_total` | Counter | — |

## Configuration

All settings via environment variables (12-factor):

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://vault:vault@localhost:5432/vault?sslmode=disable` | Postgres DSN |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password |
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `GRPC_ADDR` | `:9090` | gRPC listen address |
| `CACHE_TTL` | `5m` | Redis cache TTL |
| `RATE_LIMIT_MAX` | `100` | Max requests per IP per window |
| `RATE_LIMIT_WINDOW` | `1m` | Rate limit window |
| `CODE_LENGTH` | `7` | Short code length |
| `LOG_LEVEL` | `info` | `debug` or `info` |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |

## Development

```bash
make build    # compile binary → bin/server
make test     # go test -race ./...
make lint     # golangci-lint
make vet      # go vet
make proto    # regenerate protobuf stubs (requires protoc + plugins)
```

## Resume keywords backed by this repo

Go · gRPC · Protocol Buffers · microservices · PostgreSQL (pgx/v5) · Redis (go-redis) · read-through cache · rate limiting · Prometheus metrics · Docker multi-stage build · distroless containers · graceful shutdown · 12-factor config · chi HTTP router · structured logging (slog) · testcontainers-compatible · GitHub Actions CI

## License

MIT © 2026 Chirag Singhal
