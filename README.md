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
| `auth.jwt_secret` | `AUTH_JWT_SECRET` | Must equal the frontend's `NEXTAUTH_SECRET` |
| `mongo.uri` | `MONGO_URI` | |
| `redis.host` / `redis.port` | `REDIS_HOST` / `REDIS_PORT` | |
| `fx.api_url` / `fx.api_key` | `FX_API_URL` / `FX_API_KEY` | Empty uses the keyless exchangerate.host default |

See `configs/config.dev.example.yaml` for the full set.

Mongo and Redis are both **optional at startup**. If either is unreachable the
API still serves search and FX; `/healthz` reports the degradation. Without
Redis, FX rates are fetched per request instead of cached.

## API

Every response uses one envelope. Exactly one of `data` and `error` is set, so
clients branch on `error !== null`:

```json
{ "data": { "...": "..." }, "error": null }
{ "data": null, "error": { "code": "INVALID_INPUT", "message": "..." } }
```

Error codes: `INVALID_INPUT`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`,
`FX_UNAVAILABLE`, `INTERNAL_SERVER_ERROR`.

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/healthz` | — | Liveness plus per-dependency status. 503 if any dependency is down |
| `GET` | `/api/v1/search?q=` | — | Product search. `q` is required |
| `GET` | `/api/v1/fx?from=&to=&amount=` | — | Currency conversion. Defaults to 1 USD→ETB |

### Not yet mounted

Alert handlers (`POST`/`GET`/`DELETE /alerts`) are implemented and tested but
deliberately **not routed**: they require an authenticated user ID, and the
auth middleware does not exist yet. Mounting them before then would let any
caller create and delete alerts belonging to anyone.

`compare_handler.go`, `compare_products.go`, `alibaba_http_gateway.go`,
`llm_http_gateway.go`, `fcm_http_gateway.go` and `platform/queue.go` are
placeholders awaiting their features.

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

### A note for Windows

Some Windows setups block Go's compiled test binaries from executing under
Application Control, which surfaces as:

```
fork/exec ...handler.test.exe: An Application Control policy has blocked this file
```

This is a local machine policy, not a test failure. CI runs on Ubuntu and
executes every package.
