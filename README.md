# HookRelay — WebHook Gateway

> Unified webhook hub: signature verification, reliable delivery, automatic retries, full traceability.

## Features

- **Multi-source ingestion** — Stripe, GitHub, Slack, or any custom webhook provider
- **Signature verification** — HMAC-SHA256, HMAC-SHA1, Basic Auth, Bearer Token
- **Reliable delivery** — PostgreSQL-backed queue with `FOR UPDATE SKIP LOCKED`
- **Automatic retries** — Exponential backoff with jitter, Dead Letter Queue fallback
- **Fan-out routing** — One event, multiple subscribers with glob pattern matching
- **Payload transformation** — JMESPath, field remapping, Go templates
- **Dashboard API** — Full REST API for sources, subscriptions, events, and delivery management
- **Observability** — Prometheus metrics, structured logging (slog), request tracing

## Quick Start

### Docker Compose (recommended)

```bash
# Start PostgreSQL + HookRelay
docker-compose -f deploy/docker-compose.yml up -d

# Apply database migrations
psql "postgresql://hookrelay:hookrelay@localhost:5432/hookrelay" -f migrations/001_init.sql
```

### From Source

```bash
# Build
make build

# Run (ensure PostgreSQL is running)
./bin/hookrelay --config config/config.yaml
```

### Try It Out

```bash
# 1. Create a source
curl -s -X POST http://localhost:8080/api/v1/sources \
  -H "Authorization: Bearer changeme" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "verify_type": "none",
    "verify_config": {},
    "event_type_path": "type"
  }' | jq .

# 2. Create a subscription
curl -s -X POST http://localhost:8080/api/v1/subscriptions \
  -H "Authorization: Bearer changeme" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "notify-backend",
    "source_id": 1,
    "event_filter": ["*"],
    "target_url": "https://httpbin.org/post",
    "signing_secret": "my-secret"
  }' | jq .

# 3. Send a webhook event
curl -s -X POST http://localhost:8080/ingest/my-app \
  -H "Content-Type: application/json" \
  -d '{"type": "order.created", "data": {"id": "ord_123", "amount": 9900}}' | jq .

# 4. Check delivery status
curl -s http://localhost:8080/api/v1/events/1/deliveries \
  -H "Authorization: Bearer changeme" | jq .
```

## Configuration

HookRelay uses a YAML config file with environment variable overrides.

```yaml
# config/config.yaml (see file for full reference)
server:
  port: 8080
database:
  dsn: "postgresql://hookrelay:hookrelay@localhost:5432/hookrelay?sslmode=disable"
delivery:
  workers: 4
  poll_interval: "1s"
```

**Environment variable overrides** use the `HOOKRELAY_` prefix:

| Variable | Config Path |
|----------|-------------|
| `HOOKRELAY_SERVER_PORT` | `server.port` |
| `HOOKRELAY_DATABASE_DSN` | `database.dsn` |
| `HOOKRELAY_API_KEY` | `api.key` |
| `HOOKRELAY_DELIVERY_WORKERS` | `delivery.workers` |

## API Overview

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/ingest/{sourceName}` | Receive webhook events |
| `GET/POST` | `/api/v1/sources` | List / Create sources |
| `GET/PUT/DELETE` | `/api/v1/sources/{id}` | Source CRUD |
| `GET/POST` | `/api/v1/subscriptions` | List / Create subscriptions |
| `GET/PUT/DELETE` | `/api/v1/subscriptions/{id}` | Subscription CRUD |
| `GET` | `/api/v1/subscriptions/{id}/stats` | Delivery statistics |
| `GET` | `/api/v1/events` | List events (filterable) |
| `GET` | `/api/v1/events/{id}` | Event detail |
| `GET` | `/api/v1/events/{id}/deliveries` | Event deliveries |
| `POST` | `/api/v1/events/{id}/replay` | Replay event |
| `GET` | `/api/v1/dead-letters` | List dead letters |
| `POST` | `/api/v1/dead-letters/{id}/retry` | Retry dead letter |
| `POST` | `/api/v1/dead-letters/batch-retry` | Batch retry |
| `GET` | `/api/v1/stats/overview` | Global statistics |
| `GET` | `/api/v1/stats/throughput` | Throughput time series |
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/readyz` | Readiness probe |
| `GET` | `/metrics` | Prometheus metrics |

## Development

```bash
make build        # Build binary
make run          # Build and run
make test         # Run tests
make lint         # Run linter
make docker-up    # Start docker-compose stack
make docker-down  # Stop docker-compose stack
```

## Architecture

See [docs/design.md](docs/design.md) for the full technical design document including data models, module design, deployment architecture, and security considerations.

## License

MIT
