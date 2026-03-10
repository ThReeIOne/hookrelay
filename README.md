# HookRelay — WebHook 网关

> 所有 webhook 的收发中枢：统一验签、可靠投递、失败重试、全程可追溯

## 一、项目概述

### 定位

HookRelay 是一个通用的 WebHook 网关服务，统一管理所有 inbound/outbound webhook 的接收、验签、路由、投递、重试和日志记录。

### 解决的问题

- 接入多个 webhook 来源（Stripe/GitHub/Slack/自定义），每个的验签方式不一样，重复写验签逻辑
- webhook 投递失败了不知道，没有重试，数据丢了
- 排查"钱扣了但是没收到回调"这类问题，没有日志可查
- 一个事件要通知多个下游（fan-out），手写管理混乱
- 下游服务偶尔挂，需要指数退避重试 + Dead Letter 兜底

### 不做什么

- 不做消息队列（RabbitMQ/Kafka 做得更好）
- 不做 Event Bus / Event Mesh（专注 HTTP webhook 场景）
- 不做流式处理

## 二、技术栈

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | **Go 1.22+** | 高并发 HTTP 处理、单二进制部署、goroutine 天然适合并行投递 |
| API | **net/http + chi router** | 标准库 + 轻量路由 |
| 存储 | **PostgreSQL 15+** | 事务保证 + JSONB + `FOR UPDATE SKIP LOCKED` 实现可靠投递队列 |
| 延迟队列 | **PostgreSQL (轻量)** | 基于 `next_attempt_at` 字段轮询，简单可靠 |
| 延迟队列 | **Redis Sorted Set (高吞吐)** | 量大时切换，score = 下次执行时间戳 |
| 限流 | **Redis 滑动窗口** | 每个 subscription 独立限流 |
| 签名 | **标准库 crypto** | HMAC-SHA256, RSA-SHA256, Ed25519 |
| 配置 | **YAML + env** | |
| 监控 | **Prometheus** | 投递成功率、延迟、队列深度 |
| 日志 | **slog** | 结构化日志 |

## 三、整体架构

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

## 四、数据模型

### 4.1 source — 事件来源

```sql
CREATE TABLE source (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(64) UNIQUE NOT NULL,   -- "stripe", "github", "my-app"
    verify_type     VARCHAR(32) NOT NULL,
    -- "hmac_sha256"   : Stripe, 自定义
    -- "hmac_sha1"     : GitHub
    -- "rsa_sha256"    : 某些支付平台
    -- "basic_auth"    : 简单认证
    -- "bearer_token"  : Token 校验
    -- "none"          : 不验签
    verify_config   JSONB NOT NULL,
    -- hmac:  {"secret": "whsec_xxx", "header": "Stripe-Signature", "tolerance": 300}
    -- github: {"secret": "xxx", "header": "X-Hub-Signature-256"}
    -- basic: {"username": "xxx", "password": "xxx"}
    -- bearer: {"token": "xxx", "header": "Authorization"}
    event_type_path VARCHAR(128) DEFAULT '',       -- 从 payload 中提取 event_type 的 JSON path
    -- stripe: "type" → payload.type = "payment_intent.succeeded"
    -- github: 从 header X-GitHub-Event 提取
    -- 自定义: "event" 或 "data.event_type"
    event_type_header VARCHAR(64) DEFAULT '',      -- 从 header 提取 event_type (优先于 path)
    idempotency_path  VARCHAR(128) DEFAULT '',     -- 去重 key 的 JSON path，如 "id"
    idempotency_header VARCHAR(64) DEFAULT '',     -- 去重 key 从 header 取
    description     TEXT DEFAULT '',
    active          BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);
```

### 4.2 event — 接收到的事件

```sql
CREATE TABLE event (
    id              BIGSERIAL PRIMARY KEY,
    source_id       BIGINT NOT NULL REFERENCES source(id),
    source_name     VARCHAR(64) NOT NULL,          -- 冗余，方便查询
    event_type      VARCHAR(128) NOT NULL,
    idempotency_key VARCHAR(256) DEFAULT '',
    headers         JSONB NOT NULL DEFAULT '{}',
    payload         JSONB NOT NULL,
    raw_body        BYTEA,                         -- 原始请求体（验签需要）
    remote_addr     VARCHAR(45),                   -- 来源 IP
    received_at     TIMESTAMPTZ DEFAULT now(),

    UNIQUE(source_id, idempotency_key) -- 去重约束（idempotency_key 非空时生效）
);

CREATE INDEX idx_event_source_type ON event(source_id, event_type, received_at DESC);
CREATE INDEX idx_event_received ON event(received_at DESC);
```

### 4.3 subscription — 订阅

```sql
CREATE TABLE subscription (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,
    source_id       BIGINT NOT NULL REFERENCES source(id),
    event_filter    JSONB NOT NULL DEFAULT '["*"]',
    -- 支持通配符: ["payment.*", "refund.*"]
    -- 精确匹配:   ["payment.succeeded"]
    -- 全部匹配:   ["*"]
    target_url      VARCHAR(512) NOT NULL,         -- 投递目标
    signing_secret  VARCHAR(256) DEFAULT '',       -- 给下游签名的密钥 (HMAC-SHA256)
    custom_headers  JSONB DEFAULT '{}',            -- 附加到投递请求的 headers
    transform       JSONB,                         -- payload 转换规则 (可选)
    -- {"type": "jmespath", "expression": "data.object"}
    -- {"type": "template", "body": "{\"order_id\": \"{{.payload.id}}\"}"}
    -- null = 原样投递
    max_retries     INT DEFAULT 8,
    timeout_seconds INT DEFAULT 30,
    rate_limit_rps  INT DEFAULT 100,               -- 每秒最大投递次数
    active          BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_sub_source ON subscription(source_id, active);
```

### 4.4 delivery — 投递记录

```sql
CREATE TABLE delivery (
    id              BIGSERIAL PRIMARY KEY,
    event_id        BIGINT NOT NULL REFERENCES event(id),
    subscription_id BIGINT NOT NULL REFERENCES subscription(id),
    status          SMALLINT NOT NULL DEFAULT 0,
    -- 0=pending  1=delivering  2=success  3=failed  4=dead_letter
    attempt_count   INT DEFAULT 0,
    max_retries     INT NOT NULL,
    next_attempt_at TIMESTAMPTZ,                   -- null = 立即投递
    last_status_code INT,
    last_response   TEXT,                          -- 截断到 2000 字符
    last_error      TEXT,
    last_duration_ms INT,                          -- 投递耗时
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ,

    UNIQUE(event_id, subscription_id)              -- 同一事件同一订阅只投递一次
);

-- 投递调度核心索引：只查 pending 和 failed
CREATE INDEX idx_delivery_pending
    ON delivery(next_attempt_at NULLS FIRST)
    WHERE status IN (0, 3);
```

### 4.5 delivery_attempt — 每次尝试的详细记录

```sql
CREATE TABLE delivery_attempt (
    id              BIGSERIAL PRIMARY KEY,
    delivery_id     BIGINT NOT NULL REFERENCES delivery(id),
    attempt_number  INT NOT NULL,
    status_code     INT,
    response_body   TEXT,                          -- 截断
    error           TEXT,
    duration_ms     INT,
    request_headers JSONB,                         -- 发出的 headers（脱敏）
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_attempt_delivery ON delivery_attempt(delivery_id, attempt_number);
```

## 五、核心模块设计

### 5.1 验签模块 — 支持多种方式

```go
package verify

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "math"
    "strconv"
    "strings"
    "time"
)

type Verifier interface {
    Verify(config map[string]any, headers map[string]string, body []byte) error
}

// 根据 verify_type 获取对应的 Verifier
func GetVerifier(verifyType string) Verifier {
    switch verifyType {
    case "hmac_sha256":
        return &HMACSha256Verifier{}
    case "hmac_sha1":
        return &HMACSha1Verifier{}
    case "basic_auth":
        return &BasicAuthVerifier{}
    case "bearer_token":
        return &BearerTokenVerifier{}
    case "none":
        return &NoopVerifier{}
    default:
        return nil
    }
}

// HMAC-SHA256 验签（兼容 Stripe 格式）
type HMACSha256Verifier struct{}

func (v *HMACSha256Verifier) Verify(config map[string]any, headers map[string]string, body []byte) error {
    secret := config["secret"].(string)
    headerName := config["header"].(string)
    sigHeader := headers[strings.ToLower(headerName)]

    if sigHeader == "" {
        return fmt.Errorf("missing signature header: %s", headerName)
    }

    // Stripe 格式: "t=1614556828,v1=xxx"
    if strings.Contains(sigHeader, "t=") {
        return v.verifyStripeFormat(secret, sigHeader, body, config)
    }

    // 普通格式: "sha256=xxx" 或直接 hex
    sig := strings.TrimPrefix(sigHeader, "sha256=")
    expected := computeHMACSHA256([]byte(secret), body)
    if !hmac.Equal([]byte(expected), []byte(sig)) {
        return fmt.Errorf("signature mismatch")
    }
    return nil
}

func (v *HMACSha256Verifier) verifyStripeFormat(secret, sigHeader string, body []byte, config map[string]any) error {
    parts := make(map[string]string)
    for _, p := range strings.Split(sigHeader, ",") {
        kv := strings.SplitN(p, "=", 2)
        if len(kv) == 2 {
            parts[kv[0]] = kv[1]
        }
    }

    timestamp := parts["t"]
    signature := parts["v1"]

    // 时间容差
    if tolerance, ok := config["tolerance"]; ok {
        ts, _ := strconv.ParseInt(timestamp, 10, 64)
        diff := math.Abs(float64(time.Now().Unix() - ts))
        if diff > tolerance.(float64) {
            return fmt.Errorf("signature timestamp too old: %.0fs", diff)
        }
    }

    payload := timestamp + "." + string(body)
    expected := computeHMACSHA256([]byte(secret), []byte(payload))
    if !hmac.Equal([]byte(expected), []byte(signature)) {
        return fmt.Errorf("signature mismatch")
    }
    return nil
}

func computeHMACSHA256(key, data []byte) string {
    h := hmac.New(sha256.New, key)
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}
```

### 5.2 Ingress — 事件接收

```go
package ingress

import (
    "encoding/json"
    "io"
    "net/http"
)

type Handler struct {
    store    Store
    router   *Router
    verifier *VerifierRegistry
}

// ServeHTTP 处理 POST /ingest/:source_name
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    sourceName := chi.URLParam(r, "sourceName")

    // 1. 查找 source 配置
    source, err := h.store.GetSourceByName(r.Context(), sourceName)
    if err != nil {
        writeJSON(w, 404, map[string]string{"error": "source not found"})
        return
    }
    if !source.Active {
        writeJSON(w, 403, map[string]string{"error": "source is disabled"})
        return
    }

    // 2. 读取原始 body
    rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
    if err != nil {
        writeJSON(w, 400, map[string]string{"error": "failed to read body"})
        return
    }

    // 3. 验签
    headers := flattenHeaders(r.Header)
    verifier := verify.GetVerifier(source.VerifyType)
    if verifier == nil {
        writeJSON(w, 500, map[string]string{"error": "unknown verify type"})
        return
    }
    if err := verifier.Verify(source.VerifyConfig, headers, rawBody); err != nil {
        writeJSON(w, 401, map[string]string{"error": "signature verification failed"})
        return
    }

    // 4. 解析 payload
    var payload map[string]any
    if err := json.Unmarshal(rawBody, &payload); err != nil {
        writeJSON(w, 400, map[string]string{"error": "invalid JSON payload"})
        return
    }

    // 5. 提取 event_type
    eventType := h.extractEventType(source, headers, payload)

    // 6. 提取幂等 key + 去重
    idempotencyKey := h.extractIdempotencyKey(source, headers, payload)
    if idempotencyKey != "" {
        existing, _ := h.store.FindEventByIdempotencyKey(r.Context(), source.ID, idempotencyKey)
        if existing != nil {
            writeJSON(w, 200, map[string]any{
                "status":   "duplicate",
                "event_id": existing.ID,
            })
            return
        }
    }

    // 7. 存储事件
    event, err := h.store.CreateEvent(r.Context(), &Event{
        SourceID:       source.ID,
        SourceName:     source.Name,
        EventType:      eventType,
        IdempotencyKey: idempotencyKey,
        Headers:        headers,
        Payload:        payload,
        RawBody:        rawBody,
        RemoteAddr:     r.RemoteAddr,
    })
    if err != nil {
        writeJSON(w, 500, map[string]string{"error": "failed to store event"})
        return
    }

    // 8. 路由 → 创建投递任务
    deliveryCount, err := h.router.Route(r.Context(), event)
    if err != nil {
        // event 已存储，路由失败不影响 ack
        slog.Error("routing failed", "event_id", event.ID, "error", err)
    }

    writeJSON(w, 200, map[string]any{
        "status":     "accepted",
        "event_id":   event.ID,
        "deliveries": deliveryCount,
    })
}

func (h *Handler) extractEventType(source *Source, headers map[string]string, payload map[string]any) string {
    // 优先从 header 取
    if source.EventTypeHeader != "" {
        if v, ok := headers[strings.ToLower(source.EventTypeHeader)]; ok {
            return v
        }
    }
    // 从 payload JSON path 取
    if source.EventTypePath != "" {
        return jsonPathGet(payload, source.EventTypePath)
    }
    return "unknown"
}
```

### 5.3 Router — 事件路由 + 通配符匹配

```go
package router

import (
    "context"
    "strings"
)

type Router struct {
    store Store
}

// Route 将事件匹配到所有符合条件的订阅，创建 delivery
func (r *Router) Route(ctx context.Context, event *Event) (int, error) {
    subs, err := r.store.ListActiveSubscriptions(ctx, event.SourceID)
    if err != nil {
        return 0, err
    }

    count := 0
    for _, sub := range subs {
        if !r.matchFilter(sub.EventFilter, event.EventType) {
            continue
        }
        _, err := r.store.CreateDelivery(ctx, &Delivery{
            EventID:        event.ID,
            SubscriptionID: sub.ID,
            Status:         StatusPending,
            MaxRetries:     sub.MaxRetries,
            NextAttemptAt:  nil, // nil = 立即
        })
        if err != nil {
            slog.Error("create delivery failed", "sub_id", sub.ID, "error", err)
            continue
        }
        count++
    }
    return count, nil
}

func (r *Router) matchFilter(filters []string, eventType string) bool {
    for _, pattern := range filters {
        if matchGlob(pattern, eventType) {
            return true
        }
    }
    return false
}

// matchGlob 支持的模式：
//   "*"              → 匹配所有
//   "payment.*"      → 匹配 payment.succeeded, payment.failed
//   "payment.**"     → 匹配 payment.intent.succeeded (多级)
//   "payment.succeeded" → 精确匹配
func matchGlob(pattern, value string) bool {
    if pattern == "*" || pattern == "**" {
        return true
    }

    patternParts := strings.Split(pattern, ".")
    valueParts := strings.Split(value, ".")

    pi, vi := 0, 0
    for pi < len(patternParts) && vi < len(valueParts) {
        if patternParts[pi] == "**" {
            return true // ** 匹配剩余所有
        }
        if patternParts[pi] == "*" {
            pi++
            vi++
            continue
        }
        if patternParts[pi] != valueParts[vi] {
            return false
        }
        pi++
        vi++
    }

    return pi == len(patternParts) && vi == len(valueParts)
}
```

### 5.4 Delivery Engine — 投递 + 重试

```go
package delivery

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "math/rand"
    "net/http"
    "time"
)

// 指数退避间隔表 (秒)
// 尝试 1: 10s, 2: 30s, 3: 1m, 4: 5m, 5: 15m, 6: 1h, 7: 4h, 8: 12h
var retryDelays = []int{10, 30, 60, 300, 900, 3600, 14400, 43200}

type Engine struct {
    store       Store
    rateLimiter *RateLimiter
    httpClient  *http.Client
    workers     int
}

// Run 启动投递引擎
func (e *Engine) Run(ctx context.Context) {
    for i := 0; i < e.workers; i++ {
        go e.pollLoop(ctx)
    }
    <-ctx.Done()
}

func (e *Engine) pollLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        deliveries, err := e.store.FetchPendingDeliveries(ctx, 50)
        // SQL: SELECT ... WHERE status IN (0, 3) AND next_attempt_at <= now()
        //      FOR UPDATE SKIP LOCKED LIMIT 50
        if err != nil {
            time.Sleep(time.Second)
            continue
        }

        if len(deliveries) == 0 {
            time.Sleep(time.Second)
            continue
        }

        // 并行投递
        var wg sync.WaitGroup
        for _, d := range deliveries {
            wg.Add(1)
            go func(d *Delivery) {
                defer wg.Done()
                e.deliver(ctx, d)
            }(d)
        }
        wg.Wait()
    }
}

func (e *Engine) deliver(ctx context.Context, d *Delivery) {
    event, _ := e.store.GetEvent(ctx, d.EventID)
    sub, _ := e.store.GetSubscription(ctx, d.SubscriptionID)

    // 限流检查
    if !e.rateLimiter.Allow(d.SubscriptionID, sub.RateLimitRPS) {
        // 稍后重试
        d.NextAttemptAt = timePtr(time.Now().Add(time.Second))
        e.store.UpdateDelivery(ctx, d)
        return
    }

    // 构造请求
    payload := e.transformPayload(event.Payload, sub.Transform)
    bodyBytes, _ := json.Marshal(payload)

    timestamp := fmt.Sprintf("%d", time.Now().Unix())
    headers := map[string]string{
        "Content-Type":          "application/json",
        "User-Agent":            "HookRelay/1.0",
        "X-HookRelay-Event-ID": fmt.Sprintf("%d", event.ID),
        "X-HookRelay-Event":    event.EventType,
        "X-HookRelay-Timestamp": timestamp,
        "X-HookRelay-Attempt":  fmt.Sprintf("%d", d.AttemptCount+1),
    }

    // 合并自定义 headers
    for k, v := range sub.CustomHeaders {
        headers[k] = v
    }

    // 签名（给下游验证用）
    if sub.SigningSecret != "" {
        sigPayload := timestamp + "." + string(bodyBytes)
        mac := hmac.New(sha256.New, []byte(sub.SigningSecret))
        mac.Write([]byte(sigPayload))
        sig := hex.EncodeToString(mac.Sum(nil))
        headers["X-HookRelay-Signature"] = fmt.Sprintf("t=%s,v1=%s", timestamp, sig)
    }

    // 发送
    start := time.Now()
    req, _ := http.NewRequestWithContext(ctx, "POST", sub.TargetURL, bytes.NewReader(bodyBytes))
    for k, v := range headers {
        req.Header.Set(k, v)
    }

    resp, err := e.httpClient.Do(req)
    durationMs := int(time.Since(start).Milliseconds())

    // 记录本次尝试
    attempt := &DeliveryAttempt{
        DeliveryID:    d.ID,
        AttemptNumber: d.AttemptCount + 1,
        DurationMs:    durationMs,
    }

    d.AttemptCount++
    d.LastDurationMs = durationMs

    if err != nil {
        // 网络错误
        attempt.Error = err.Error()
        d.LastError = truncate(err.Error(), 2000)
        e.scheduleRetry(d)
    } else {
        defer resp.Body.Close()
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

        attempt.StatusCode = resp.StatusCode
        attempt.ResponseBody = string(body)
        d.LastStatusCode = resp.StatusCode
        d.LastResponse = truncate(string(body), 2000)

        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            d.Status = StatusSuccess
            d.CompletedAt = timePtr(time.Now())
        } else {
            e.scheduleRetry(d)
        }
    }

    e.store.CreateDeliveryAttempt(ctx, attempt)
    e.store.UpdateDelivery(ctx, d)
}

func (e *Engine) scheduleRetry(d *Delivery) {
    if d.AttemptCount >= d.MaxRetries {
        d.Status = StatusDeadLetter
        d.CompletedAt = timePtr(time.Now())
        return
    }

    d.Status = StatusFailed
    idx := min(d.AttemptCount-1, len(retryDelays)-1)
    delay := retryDelays[idx]
    // 加 10% 随机抖动，防止重试风暴
    jitter := float64(delay) * 0.1 * rand.Float64()
    d.NextAttemptAt = timePtr(time.Now().Add(time.Duration(float64(delay)+jitter) * time.Second))
}
```

### 5.5 限流 — Redis 滑动窗口

```go
package ratelimit

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type RateLimiter struct {
    redis *redis.Client
}

// Allow 检查某个 subscription 是否可以投递
func (r *RateLimiter) Allow(subscriptionID int64, rps int) bool {
    if rps <= 0 {
        return true
    }

    key := fmt.Sprintf("hookrelay:ratelimit:%d", subscriptionID)
    now := time.Now().UnixMicro()
    windowStart := now - int64(time.Second/time.Microsecond)

    // Lua 脚本：原子操作，滑动窗口
    script := `
        redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
        local count = redis.call('ZCARD', KEYS[1])
        if count < tonumber(ARGV[2]) then
            redis.call('ZADD', KEYS[1], ARGV[3], ARGV[3])
            redis.call('EXPIRE', KEYS[1], 2)
            return 1
        end
        return 0
    `

    result, err := r.redis.Eval(context.Background(), script,
        []string{key},
        windowStart, rps, now,
    ).Int()

    if err != nil {
        return true // Redis 异常时放行
    }
    return result == 1
}
```

### 5.6 Dashboard API

```go
package api

// ========== Source 管理 ==========

// POST   /api/v1/sources              创建来源
// GET    /api/v1/sources              列出所有来源
// GET    /api/v1/sources/:id          来源详情
// PUT    /api/v1/sources/:id          更新来源
// DELETE /api/v1/sources/:id          禁用来源

// ========== Subscription 管理 ==========

// POST   /api/v1/subscriptions              创建订阅
// GET    /api/v1/subscriptions              列出订阅
// GET    /api/v1/subscriptions/:id          订阅详情
// PUT    /api/v1/subscriptions/:id          更新订阅
// DELETE /api/v1/subscriptions/:id          禁用订阅
// GET    /api/v1/subscriptions/:id/stats    投递统计
//        → {total, success, failed, dead_letter, avg_latency_ms, p99_latency_ms, success_rate}

// ========== Event 查询 ==========

// GET    /api/v1/events
//        ?source=stripe
//        &event_type=payment.succeeded
//        &start=2024-01-01T00:00:00Z
//        &end=2024-01-02T00:00:00Z
//        &page=1&page_size=20

// GET    /api/v1/events/:id                 事件详情（含 payload）
// GET    /api/v1/events/:id/deliveries      该事件的所有投递记录

// ========== 事件重放 ==========

// POST   /api/v1/events/:id/replay
//        Body: {"subscription_ids": [1, 2]}  可选，空则重放给所有订阅
//        → 重新创建 delivery 记录

// ========== Dead Letter 管理 ==========

// GET    /api/v1/dead-letters
//        ?subscription_id=1
//        &page=1&page_size=20

// POST   /api/v1/dead-letters/:delivery_id/retry
//        → 重置 delivery 状态为 pending，立即重新投递

// POST   /api/v1/dead-letters/batch-retry
//        Body: {"delivery_ids": [1, 2, 3]}

// ========== 全局统计 ==========

// GET    /api/v1/stats/overview
//        → {events_today, deliveries_today, success_rate, avg_latency_ms,
//           dead_letters_pending, active_sources, active_subscriptions}

// GET    /api/v1/stats/throughput
//        ?start=...&end=...&granularity=1h
//        → [{timestamp, events, deliveries, success, failed}]
```

## 六、Payload 转换

```go
package transform

// 支持三种转换方式
type Transformer interface {
    Transform(payload map[string]any) (map[string]any, error)
}

func GetTransformer(config map[string]any) Transformer {
    if config == nil {
        return &PassthroughTransformer{}
    }
    switch config["type"] {
    case "jmespath":
        // JMESPath 表达式提取子集
        // 例: {"type": "jmespath", "expression": "data.object"}
        return &JMESPathTransformer{Expression: config["expression"].(string)}
    case "remap":
        // 字段重映射
        // 例: {"type": "remap", "mapping": {"orderId": "data.order.id", "amount": "data.amount"}}
        return &RemapTransformer{Mapping: config["mapping"].(map[string]any)}
    case "template":
        // Go template
        // 例: {"type": "template", "body": "{\"id\": \"{{.id}}\", \"event\": \"{{.type}}\"}"}
        return &TemplateTransformer{Template: config["body"].(string)}
    default:
        return &PassthroughTransformer{}
    }
}
```

## 七、项目结构

```
hookrelay/
├── cmd/
│   └── hookrelay/
│       └── main.go              # 服务入口
├── internal/
│   ├── ingress/
│   │   └── handler.go           # 事件接收
│   ├── verify/
│   │   ├── verify.go            # Verifier 接口
│   │   ├── hmac_sha256.go
│   │   ├── hmac_sha1.go
│   │   ├── basic_auth.go
│   │   ├── bearer_token.go
│   │   └── noop.go
│   ├── router/
│   │   ├── router.go            # 事件路由
│   │   └── glob.go              # 通配符匹配
│   ├── delivery/
│   │   ├── engine.go            # 投递引擎
│   │   ├── retry.go             # 重试策略
│   │   └── signer.go            # 出站签名
│   ├── transform/
│   │   ├── transform.go         # 接口
│   │   ├── jmespath.go
│   │   ├── remap.go
│   │   └── template.go
│   ├── ratelimit/
│   │   └── sliding_window.go    # Redis 滑动窗口限流
│   ├── store/
│   │   ├── store.go             # 接口定义
│   │   ├── postgres.go          # PostgreSQL 实现
│   │   └── queries.go           # SQL 查询
│   └── api/
│       ├── server.go            # Dashboard HTTP Server
│       ├── handler_source.go
│       ├── handler_subscription.go
│       ├── handler_event.go
│       ├── handler_delivery.go
│       ├── handler_dead_letter.go
│       └── handler_stats.go
├── migrations/
│   └── 001_init.sql
├── config/
│   └── config.yaml
├── deploy/
│   └── docker-compose.yml       # HookRelay + PostgreSQL + Redis
├── examples/
│   ├── stripe/                  # Stripe 接入示例
│   ├── github/                  # GitHub 接入示例
│   └── custom/                  # 自定义来源示例
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 八、部署架构

```
最小部署（单机）:
  1 个 HookRelay 进程 = API + Ingress + Delivery Engine 一体化
  1 个 PostgreSQL
  1 个 Redis

水平扩展:
  N 个 HookRelay 实例（无状态）
  - Ingress: 负载均衡器分发
  - Delivery: FOR UPDATE SKIP LOCKED 保证不重复投递
  - 每个实例独立 poll，天然水平扩展
```

## 九、关键 Metrics

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

## 十、安全考虑

1. **验签严格**：所有 inbound 请求必须通过验签，不允许 `none` 类型在生产环境使用
2. **Secret 加密存储**：`signing_secret` 和 `verify_config.secret` 在数据库中加密存储（AES-GCM）
3. **出站签名**：投递给下游时带签名，下游可以验证来源
4. **请求限制**：请求体限制 1MB，响应体记录限制 2KB
5. **IP 白名单**：可选，限制只接受特定来源 IP
6. **TLS**：所有出站投递强制 HTTPS
7. **日志脱敏**：不在日志中打印完整的 secret 和 payload
