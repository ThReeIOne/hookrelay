<div align="center">

<img src="https://img.shields.io/github/stars/ThReeIOne/hookrelay?style=social" alt="GitHub stars">
<img src="https://img.shields.io/github/license/ThReeIOne/hookrelay" alt="License">
<img src="https://img.shields.io/badge/language-Go-00ADD8?logo=go" alt="Go">
<img src="https://img.shields.io/badge/database-PostgreSQL-336791?logo=postgresql" alt="PostgreSQL">
<img src="https://img.shields.io/badge/docker-ready-2496ED?logo=docker" alt="Docker">

# 🪝 HookRelay

**A self-hosted webhook gateway — reliable delivery, zero missed events.**

**自托管 Webhook 网关 —— 可靠投递，零消息丢失。**

[English](#english) · [中文](#中文) · [Quick Start](#quick-start--快速开始)

</div>

---

## English

### What is HookRelay?

HookRelay is a self-hosted webhook gateway that sits between your providers (Stripe, GitHub, Slack, etc.) and your backend services. It handles all the hard parts so your services don't have to.

**Stop debugging lost webhooks at 2AM. Let HookRelay handle it.**

### Why HookRelay?

| Problem | HookRelay Solution |
|---|---|
| Webhook signature validation is repetitive | Built-in HMAC-SHA256, HMAC-SHA1, Bearer Token, Basic Auth |
| One webhook, multiple services need it | Fan-out routing with glob pattern matching |
| Downstream service is down, events get lost | PostgreSQL-backed queue with automatic retries |
| No visibility into what failed | Full event history, delivery logs, dead letter queue |
| Payload format mismatch between services | JMESPath transformation + Go templates |

### Features

- 🔀 **Multi-source ingestion** — Stripe, GitHub, Slack, or any custom provider
- 🔐 **Signature verification** — HMAC-SHA256, HMAC-SHA1, Basic Auth, Bearer Token
- 📬 **Reliable delivery** — PostgreSQL-backed queue with `FOR UPDATE SKIP LOCKED`
- 🔁 **Automatic retries** — Exponential backoff with jitter + Dead Letter Queue
- 📡 **Fan-out routing** — One event → multiple subscribers with glob pattern matching
- 🔧 **Payload transformation** — JMESPath, field remapping, Go templates
- 📊 **Dashboard API** — Full REST API for sources, subscriptions, events, and deliveries
- 🔭 **Observability** — Prometheus metrics, structured logging (slog), request tracing

---

## 中文

### HookRelay 是什么？

HookRelay 是一个**自托管的 Webhook 网关**，部署在你的 Webhook 提供方（Stripe、GitHub、Slack 等）和后端服务之间。它统一处理所有复杂的事情，让你的业务服务专注于业务逻辑。

**不再在凌晨两点调试丢失的 Webhook，让 HookRelay 替你把关。**

### 为什么选择 HookRelay？

| 痛点 | HookRelay 解决方案 |
|---|---|
| 每个服务都要重复写签名验证 | 内置 HMAC-SHA256、HMAC-SHA1、Bearer Token、Basic Auth |
| 一个事件需要通知多个下游服务 | Fan-out 路由，支持 Glob 模式匹配 |
| 下游服务挂了，事件就丢了 | PostgreSQL 队列持久化 + 自动重试 |
| 不知道哪个 Webhook 失败了 | 完整事件历史、投递日志、死信队列 |
| 上下游 Payload 格式不一致 | JMESPath 转换 + Go 模板 |

### 核心功能

- 🔀 **多来源接入** — 支持 Stripe、GitHub、Slack 或任意自定义来源
- 🔐 **签名验证** — HMAC-SHA256、HMAC-SHA1、Basic Auth、Bearer Token
- 📬 **可靠投递** — PostgreSQL 队列，`FOR UPDATE SKIP LOCKED` 防重复消费
- 🔁 **自动重试** — 指数退避 + 抖动，失败进死信队列
- 📡 **Fan-out 路由** — 一个事件分发给多个订阅方，支持 Glob 匹配
- 🔧 **Payload 转换** — JMESPath、字段映射、Go 模板
- 📊 **管理 API** — 完整 REST API 管理来源、订阅、事件和投递记录
- 🔭 **可观测性** — Prometheus 指标、结构化日志 (slog)、请求追踪

---

## Quick Start / 快速开始

### Docker Compose（推荐 / Recommended）

```bash
# Start PostgreSQL + HookRelay / 启动 PostgreSQL + HookRelay
docker-compose -f deploy/docker-compose.yml up -d

# Apply database migrations / 执行数据库迁移
psql "postgresql://hookrelay:hookrelay@localhost:5432/hookrelay" -f migrations/001_init.sql
```

### From Source / 从源码构建

```bash
# Build / 构建
make build

# Run (ensure PostgreSQL is running) / 运行（需要先启动 PostgreSQL）
./bin/hookrelay --config config/config.yaml
```

### Try It Out / 试一试

```bash
# 1. Create a source / 创建来源
curl -s -X POST http://localhost:8080/api/v1/sources \
  -H "Authorization: Bearer changeme" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "verify_type": "none",
    "verify_config": {},
    "event_type_path": "type"
  }' | jq .

# 2. Create a subscription / 创建订阅
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

# 3. Send a webhook event / 发送一个 Webhook 事件
curl -s -X POST http://localhost:8080/ingest/my-app \
  -H "Content-Type: application/json" \
  -d '{"type": "order.created", "data": {"id": "ord_123", "amount": 9900}}' | jq .

# 4. Check delivery status / 查看投递状态
curl -s http://localhost:8080/api/v1/events/1/deliveries \
  -H "Authorization: Bearer changeme" | jq .
```

---

## Configuration / 配置

HookRelay uses a YAML config file with environment variable overrides.

HookRelay 使用 YAML 配置文件，支持环境变量覆盖。

```yaml
# config/config.yaml
server:
  port: 8080
database:
  dsn: "postgresql://hookrelay:hookrelay@localhost:5432/hookrelay?sslmode=disable"
delivery:
  workers: 4
  poll_interval: "1s"
```

| Environment Variable / 环境变量 | Config Path / 配置路径 |
|---|---|
| `HOOKRELAY_SERVER_PORT` | `server.port` |
| `HOOKRELAY_DATABASE_DSN` | `database.dsn` |
| `HOOKRELAY_API_KEY` | `api.key` |
| `HOOKRELAY_DELIVERY_WORKERS` | `delivery.workers` |

---

## API Overview / API 总览

| Method | Path | Description / 说明 |
|---|---|---|
| `POST` | `/ingest/{sourceName}` | Receive webhook events / 接收 Webhook 事件 |
| `GET/POST` | `/api/v1/sources` | List / Create sources / 列表 / 创建来源 |
| `GET/PUT/DELETE` | `/api/v1/sources/{id}` | Source CRUD / 来源增删改查 |
| `GET/POST` | `/api/v1/subscriptions` | List / Create subscriptions / 列表 / 创建订阅 |
| `GET/PUT/DELETE` | `/api/v1/subscriptions/{id}` | Subscription CRUD / 订阅增删改查 |
| `GET` | `/api/v1/subscriptions/{id}/stats` | Delivery statistics / 投递统计 |
| `GET` | `/api/v1/events` | List events / 事件列表 |
| `GET` | `/api/v1/events/{id}` | Event detail / 事件详情 |
| `GET` | `/api/v1/events/{id}/deliveries` | Event deliveries / 投递记录 |
| `POST` | `/api/v1/events/{id}/replay` | Replay event / 重放事件 |
| `GET` | `/api/v1/dead-letters` | List dead letters / 死信列表 |
| `POST` | `/api/v1/dead-letters/{id}/retry` | Retry dead letter / 重试死信 |
| `POST` | `/api/v1/dead-letters/batch-retry` | Batch retry / 批量重试 |
| `GET` | `/api/v1/stats/overview` | Global statistics / 全局统计 |
| `GET` | `/api/v1/stats/throughput` | Throughput time series / 吞吐量时序 |
| `GET` | `/healthz` | Liveness probe / 存活探针 |
| `GET` | `/readyz` | Readiness probe / 就绪探针 |
| `GET` | `/metrics` | Prometheus metrics / Prometheus 指标 |

---

## Architecture / 架构

See [docs/design.md](docs/design.md) for the full technical design document including data models, module design, deployment architecture, and security considerations.

完整技术设计文档见 [docs/design.md](docs/design.md)，包含数据模型、模块设计、部署架构和安全注意事项。

---

## Development / 开发

```bash
make build        # Build binary / 构建二进制
make run          # Build and run / 构建并运行
make test         # Run tests / 运行测试
make lint         # Run linter / 代码检查
make docker-up    # Start docker-compose stack / 启动 Docker 环境
make docker-down  # Stop docker-compose stack / 停止 Docker 环境
```

---

## Contributing / 贡献

Issues and PRs are welcome! / 欢迎提 Issue 和 PR！

If HookRelay saves you from debugging lost webhooks, please consider giving it a ⭐

如果 HookRelay 帮你解决了 Webhook 丢失的问题，欢迎点个 ⭐ 支持一下！

---

## License

MIT © [ThReeIOne](https://github.com/ThReeIOne)
