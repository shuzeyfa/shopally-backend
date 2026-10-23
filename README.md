# ShopAlly Backend

Go API for ShopAlly — an AI shopping assistant for AliExpress, surfacing
products with AI match scoring, dual ETB/USD pricing, comparison, saved items
and price alerts.

Clean architecture: `pkg/domain` and `pkg/usecase` hold business logic with no
framework imports; `internal/adapter` holds the HTTP, gateway and repository
adapters; `internal/platform` holds driver setup.

## Quick start

```bash
# 1. Dependencies (Mongo + Redis)
docker compose up -d

# 2. Config — optional, defaults already target docker-compose
cp configs/config.dev.example.yaml configs/config.dev.yaml

# 3. Run
go run ./cmd/api
```

The API listens on `:8080`. Verify:

```bash
curl localhost:8080/healthz
curl 'localhost:8080/api/v1/search?q=phone'
curl 'localhost:8080/api/v1/fx?from=USD&to=ETB&amount=10'
```

## Configuration

Config is read from `configs/config.dev.yaml` if present, then overlaid with
environment variables. **Neither is required** — defaults boot a working local
instance.

Environment variables use the upper-cased config path with dots replaced by
underscores, and override the file. Hosted deploys use these exclusively:

| Config key | Environment variable | Notes |
|---|---|---|
| `server.port` | `SERVER_PORT` | Default `8080` |
| `server.env` | `SERVER_ENV` | Set `production` when deployed |
| `cors.allowed_origins` | `CORS_ALLOWED_ORIGINS` | Comma-separated. **Required** for a browser frontend |
| `auth.jwt_secret` | `AUTH_JWT_SECRET` | Must equal the frontend's dedicated `AUTH_JWT_SECRET` |
| `mongo.uri` | `MONGO_URI` | |
| `redis.host` / `redis.port` | `REDIS_HOST` / `REDIS_PORT` | Set `REDIS_TLS=true` for TLS-protected providers such as Upstash |
| `fx.api_url` / `fx.api_key` | `FX_API_URL` / `FX_API_KEY` | Configure a supported provider; the legacy default now requires authentication |

See `configs/config.dev.example.yaml` for the full set.

Mongo and Redis are both **optional at startup**. The API starts listening
immediately and connects to each dependency independently in the background.
Failed initial connections retry every five seconds after each bounded attempt.
When a connection succeeds, repositories and caching become available without
a restart. Once connected, the drivers handle subsequent network outages.
The worker uses the same recovery behavior and warms FX when Redis connects.

`/healthz` returns `503` while either dependency is unavailable and `200` once
both are reachable. Search remains available during an outage; authenticated
saved-item and alert requests return `503` while Mongo is unavailable. Without
Redis, FX calls bypass caching and still require a working provider. Shutdown
cancels pending connection attempts and worker tasks before releasing clients.

Missing config files are supported; malformed or unreadable files are errors.
Local `.env` files are not loaded automatically: export environment variables
or use the optional YAML file. Docker excludes `.env`, its variants, and the
local development YAML from the build context.

## API

Every response uses one envelope. Exactly one of `data` and `error` is set, so
clients branch on `error !== null`:

```json
{ "data": { "...": "..." }, "error": null }
{ "data": null, "error": { "code": "INVALID_INPUT", "message": "..." } }
```

Error codes: `INVALID_INPUT`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`,
`FX_UNAVAILABLE`, `SERVICE_UNAVAILABLE`, `INTERNAL_SERVER_ERROR`.

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/healthz` | — | Liveness plus per-dependency status. 503 if any dependency is down |
| `GET` | `/api/v1/search?q=` | — | Product search. `q` is required |
| `GET` | `/api/v1/fx?from=&to=&amount=` | — | Currency conversion. Defaults to 1 USD→ETB |
| `POST` | `/api/v1/compare` | — | Compare 2–4 products by ID |
| `GET` | `/api/v1/saved` | Bearer token | List the caller's saved products |
| `POST` | `/api/v1/saved` | Bearer token | Save or refresh a product |
| `DELETE` | `/api/v1/saved/:productId` | Bearer token | Remove a saved product |
| `GET` | `/api/v1/alerts` | Bearer token | List the caller's price alerts |
| `POST` | `/api/v1/alerts` | Bearer token | Create an alert with `productId`, `targetPrice`, and optional `currency` (`ETB` or `USD`) |
| `GET` | `/api/v1/alerts/:id` | Bearer token | Get one of the caller's alerts |
| `DELETE` | `/api/v1/alerts/:id` | Bearer token | Remove one of the caller's alerts |

### Price alerts

Alerts are stored in the configured Mongo collection (`mongo.alert_collection`).
The worker checks active targets once per minute and records when a target is
reached; the alerts page refreshes periodically to show that status. Product
prices are still mocked, and push notifications remain future work with the
FCM adapter.

The AliExpress, LLM, and FCM HTTP adapters and `platform/queue.go` remain
placeholders for their later integration work.

## Product data is mocked

`MockAlibabaGateway` and `MockLLMGateway` return fixed products and a fixed
parsed intent. Both sit behind the `AlibabaGateway` and `LLMGateway` interfaces
in `pkg/usecase/interfaces.go`, so swapping in real adapters is a change to the
construction in `cmd/api/main.go` and nothing else.

FX is **real** — `FXHTTPGateway` calls a live provider and handles four
response shapes, wrapped by `CachedFXClient` for write-through Redis caching.

## Development

```bash
make test        # gotestsum, race detector
make tidy        # go mod tidy
go vet ./...
gofmt -l .       # CI fails on any output
```

### Worker

`cmd/worker` pre-warms the USD→ETB rate into Redis every 30 minutes. It is
optional for local development.

```bash
go run ./cmd/worker
```

## Deploy

`Dockerfile` builds both the API and worker executables. `render.yaml` defines
a Render web service with `/healthz` checks and a separate background worker;
the worker uses the Starter plan, which may incur charges. Review billing and
configure MongoDB Atlas and Upstash Redis credentials before creating the
Blueprint. Set `CORS_ALLOWED_ORIGINS` to the exact Vercel production origin and
use the same `AUTH_JWT_SECRET` as the frontend. Do not commit provider URLs or
secrets to this repository.

To build and smoke-test the API image locally:

```bash
docker build -t shopally-backend .
docker run --rm -p 10000:10000 -e SERVER_PORT=10000 shopally-backend
```

### A note for Windows

Some Windows setups block Go's compiled test binaries from executing under
Application Control, which surfaces as:

```
fork/exec ...handler.test.exe: An Application Control policy has blocked this file
```

This is a local machine policy, not a test failure. CI runs on Ubuntu and
executes every package.
