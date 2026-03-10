# HookRelay 技术设计文档

> 所有 webhook 的收发中枢：统一验签、可靠投递、失败重试、全程可追溯

## 一、技术栈

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | **Go 1.22+** | 高并发 HTTP 处理、单二进制部署、goroutine 天然适合并行投递 |
| API | **net/http + chi router** | 标准库 + 轻量路由 |
| 存储 | **PostgreSQL 15+** | 事务保证 + JSONB + `FOR UPDATE SKIP LOCKED` 实现可靠投递队列 |
| 延迟队列 | **PostgreSQL (轻量)** | 基于 `next_attempt_at` 字段轮询，简单可靠 |
| 延迟队列 | **Redis Sorted Set (高吞吐)** | 量大时切换，score = 下次执行时间戳 |
| 限流 | **内存滑动窗口 / Redis** | 每个 subscription 独立限流 |
| 签名 | **标准库 crypto** | HMAC-SHA256, HMAC-SHA1 |
| 配置 | **YAML + env** | |
| 监控 | **Prometheus** | 投递成功率、延迟、队列深度 |
| 日志 | **slog** | 结构化日志 |

## 二、整体架构

```
              Inbound                                           Outbound
           (接收 webhook)                                     (投递 webhook)

┌──────────────┐
│  Stripe      │──┐
│  GitHub      │──┤
│  Slack       │──┤
│  自定义来源   │──┤
└──────────────┘  │
                  ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                           HookRelay                                     │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                        Ingress Layer                             │   │
│  │                                                                  │   │
│  │  POST /ingest/:source_name                                      │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐                │   │
│  │  │ Signature  │→ │ Idempotency│→ │ Event Type │                │   │
│  │  │ Verify     │  │ Check      │  │ Extract    │                │   │
│  │  └────────────┘  └────────────┘  └─────┬──────┘                │   │
│  └────────────────────────────────────────┼─────────────────────────┘   │
│                                           │                             │
│  ┌────────────────────────────────────────▼─────────────────────────┐   │
│  │                     PostgreSQL                                   │   │
│  │                                                                  │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────────┐  ┌────────────┐  │   │
│  │  │  source  │  │  event   │  │ subscription │  │  delivery  │  │   │
│  │  └──────────┘  └──────────┘  └──────────────┘  └────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                           │                             │
│  ┌────────────────────────────────────────▼─────────────────────────┐   │
│  │                     Router                                       │   │
│  │                                                                  │   │
│  │  Event → 匹配 subscription (通配符) → 创建 delivery 记录         │   │
│  └────────────────────────────────────────┬─────────────────────────┘   │
│                                           │                             │
│  ┌────────────────────────────────────────▼─────────────────────────┐   │
│  │                  Delivery Engine                                  │   │
│  │                                                                  │   │
│  │  ┌───────────────┐    ┌───────────────┐    ┌──────────────────┐ │   │
│  │  │ Poll pending  │ →  │ Rate Limit    │ →  │ HTTP POST        │─┼──▶ 客户 A
│  │  │ deliveries    │    │ per sub       │    │ + HMAC Sign      │─┼──▶ 客户 B
│  │  │ (SKIP LOCKED) │    │               │    │ + Record result  │─┼──▶ 客户 C
│  │  └───────────────┘    └───────────────┘    └──────┬───────────┘ │   │
│  │                                                   │             │   │
│  │                                          ┌────────▼──────────┐  │   │
│  │                                          │ Retry / DLQ       │  │   │
│  │                                          │ 指数退避 + 抖动    │  │   │
│  │                                          │ 超过上限 → DLQ    │  │   │
│  │                                          └───────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                     Dashboard API                                │   │
│  │                                                                  │   │
│  │  · 事件搜索 / 详情                                               │   │
│  │  · 投递日志 / 成功率统计                                         │   │
│  │  · Dead Letter 查看 / 手动重试                                   │   │
│  │  · 事件重放                                                      │   │
│  │  · Source / Subscription CRUD                                    │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

## 三、数据模型

### 3.1 source — 事件来源

```sql
CREATE TABLE source (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(64) UNIQUE NOT NULL,
    verify_type     VARCHAR(32) NOT NULL,
    verify_config   JSONB NOT NULL,
    event_type_path VARCHAR(128) DEFAULT '',
    event_type_header VARCHAR(64) DEFAULT '',
    idempotency_path  VARCHAR(128) DEFAULT '',
    idempotency_header VARCHAR(64) DEFAULT '',
    description     TEXT DEFAULT '',
    active          BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
```

**verify_type 枚举**:
- `hmac_sha256`: Stripe、自定义 — config: `{"secret": "whsec_xxx", "header": "Stripe-Signature", "tolerance": 300}`
- `hmac_sha1`: GitHub — config: `{"secret": "xxx", "header": "X-Hub-Signature-256"}`
- `basic_auth`: 简单认证 — config: `{"username": "xxx", "password": "xxx"}`
- `bearer_token`: Token 校验 — config: `{"token": "xxx", "header": "Authorization"}`
- `none`: 不验签（仅用于开发/测试）

### 3.2 event — 接收到的事件

```sql
CREATE TABLE event (
    id              BIGSERIAL PRIMARY KEY,
    source_id       BIGINT NOT NULL REFERENCES source(id),
    source_name     VARCHAR(64) NOT NULL,
    event_type      VARCHAR(128) NOT NULL,
    idempotency_key VARCHAR(256),          -- NULL 表示无幂等 key
    headers         JSONB NOT NULL DEFAULT '{}',
    payload         JSONB NOT NULL,
    raw_body        BYTEA,
    remote_addr     VARCHAR(45),
    received_at     TIMESTAMPTZ DEFAULT now(),

    UNIQUE(source_id, idempotency_key)
);
```

### 3.3 subscription — 订阅

```sql
CREATE TABLE subscription (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,
    source_id       BIGINT NOT NULL REFERENCES source(id),
    event_filter    JSONB NOT NULL DEFAULT '["*"]',
    target_url      VARCHAR(512) NOT NULL,
    signing_secret  VARCHAR(256) DEFAULT '',
    custom_headers  JSONB DEFAULT '{}',
    transform       JSONB,
    max_retries     INT DEFAULT 8,
    timeout_seconds INT DEFAULT 30,
    rate_limit_rps  INT DEFAULT 100,
    active          BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
```

**event_filter 通配符**:
- `["*"]` — 匹配所有事件
- `["payment.*"]` — 匹配 payment.succeeded, payment.failed
- `["payment.**"]` — 匹配 payment.intent.succeeded (多级)
- `["payment.succeeded"]` — 精确匹配

### 3.4 delivery — 投递记录

```sql
CREATE TABLE delivery (
    id              BIGSERIAL PRIMARY KEY,
    event_id        BIGINT NOT NULL REFERENCES event(id),
    subscription_id BIGINT NOT NULL REFERENCES subscription(id),
    status          SMALLINT NOT NULL DEFAULT 0,
    -- 0=pending  1=delivering  2=success  3=failed  4=dead_letter
    attempt_count   INT DEFAULT 0,
    max_retries     INT NOT NULL,
    next_attempt_at TIMESTAMPTZ,
    last_status_code INT,
    last_response   TEXT,
    last_error      TEXT,
    last_duration_ms INT,
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ,

    UNIQUE(event_id, subscription_id)
);
```

### 3.5 delivery_attempt — 每次尝试的详细记录

```sql
CREATE TABLE delivery_attempt (
    id              BIGSERIAL PRIMARY KEY,
    delivery_id     BIGINT NOT NULL REFERENCES delivery(id),
    attempt_number  INT NOT NULL,
    status_code     INT,
    response_body   TEXT,
    error           TEXT,
    duration_ms     INT,
    request_headers JSONB,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

## 四、核心模块设计

### 4.1 验签模块

支持多种验签方式：HMAC-SHA256（兼容 Stripe `t=,v1=` 格式）、HMAC-SHA1（GitHub 格式）、Basic Auth、Bearer Token、Noop。

### 4.2 Ingress — 事件接收

`POST /ingest/:source_name` 完整流程：
1. 查找 source 配置
2. 读取原始 body (1MB 限制)
3. 验签
4. 解析 JSON payload
5. 提取 event_type（header 优先，fallback payload path）
6. 提取幂等 key + 去重
7. 存储事件
8. 路由 → 创建投递任务
9. 返回 200 accepted

### 4.3 Router — 事件路由

将事件匹配到所有符合条件的订阅，支持通配符匹配，创建 delivery 记录。

### 4.4 Delivery Engine — 投递 + 重试

- 多 worker poll 循环
- `FOR UPDATE SKIP LOCKED` 取任务，保证不重复投递
- 并发投递，带 per-delivery context timeout
- 出站 HMAC-SHA256 签名（`X-HookRelay-Signature: t=...,v1=...`）

**指数退避间隔**：10s, 30s, 1m, 5m, 15m, 1h, 4h, 12h + 10% 随机抖动，超限进 DLQ。

### 4.5 Payload 转换

支持四种模式：
- **passthrough**: 原样透传
- **jmespath**: JMESPath 表达式提取子集
- **remap**: 字段重映射
- **template**: Go template 渲染

### 4.6 Dashboard API

完整的 RESTful API：
- Source CRUD
- Subscription CRUD + Stats
- Event 查询 + 重放
- Dead Letter 列表 / 重试 / 批量重试
- 全局统计 + 吞吐量时序

## 五、部署架构

```
最小部署（单机）:
  1 个 HookRelay 进程 = API + Ingress + Delivery Engine 一体化
  1 个 PostgreSQL
  1 个 Redis（可选）

水平扩展:
  N 个 HookRelay 实例（无状态）
  - Ingress: 负载均衡器分发
  - Delivery: FOR UPDATE SKIP LOCKED 保证不重复投递
  - 每个实例独立 poll，天然水平扩展
```

## 六、关键 Metrics

| Metric | 类型 | 说明 |
|--------|------|------|
| `hookrelay_events_received_total` | Counter | 收到的事件总数（按 source 分标签） |
| `hookrelay_events_rejected_total` | Counter | 验签失败/去重拒绝的事件数 |
| `hookrelay_deliveries_total` | Counter | 投递次数（按 status 分标签） |
| `hookrelay_delivery_duration_ms` | Histogram | 投递耗时分布 |
| `hookrelay_delivery_queue_depth` | Gauge | 待投递队列深度 |
| `hookrelay_dead_letters_total` | Counter | 进入 Dead Letter 的数量 |
| `hookrelay_retry_total` | Counter | 重试次数 |
| `hookrelay_rate_limited_total` | Counter | 被限流的次数 |

## 七、安全考虑

1. **验签严格**：所有 inbound 请求必须通过验签，不允许 `none` 类型在生产环境使用
2. **Secret 加密存储**：`signing_secret` 和 `verify_config.secret` 在数据库中加密存储（AES-GCM）
3. **出站签名**：投递给下游时带签名，下游可以验证来源
4. **请求限制**：请求体限制 1MB，响应体记录限制 2KB
5. **IP 白名单**：可选，限制只接受特定来源 IP
6. **TLS**：所有出站投递强制 HTTPS（可配置关闭）
7. **日志脱敏**：不在日志中打印完整的 secret 和 payload
