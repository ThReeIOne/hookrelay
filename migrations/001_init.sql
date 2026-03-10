-- HookRelay schema initialization

CREATE TABLE IF NOT EXISTS source (
    id                 BIGSERIAL PRIMARY KEY,
    name               VARCHAR(64) UNIQUE NOT NULL,
    verify_type        VARCHAR(32) NOT NULL,
    verify_config      JSONB NOT NULL DEFAULT '{}',
    event_type_path    VARCHAR(128) DEFAULT '',
    event_type_header  VARCHAR(64) DEFAULT '',
    idempotency_path   VARCHAR(128) DEFAULT '',
    idempotency_header VARCHAR(64) DEFAULT '',
    description        TEXT DEFAULT '',
    active             BOOLEAN DEFAULT true,
    created_at         TIMESTAMPTZ DEFAULT now(),
    updated_at         TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS event (
    id               BIGSERIAL PRIMARY KEY,
    source_id        BIGINT NOT NULL REFERENCES source(id),
    source_name      VARCHAR(64) NOT NULL,
    event_type       VARCHAR(128) NOT NULL,
    idempotency_key  VARCHAR(256),
    headers          JSONB NOT NULL DEFAULT '{}',
    payload          JSONB NOT NULL,
    raw_body         BYTEA,
    remote_addr      VARCHAR(45),
    received_at      TIMESTAMPTZ DEFAULT now()
);

-- Partial unique index: only enforce uniqueness when idempotency_key IS NOT NULL
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_idempotency
    ON event(source_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_event_source_type
    ON event(source_id, event_type, received_at DESC);

CREATE INDEX IF NOT EXISTS idx_event_received
    ON event(received_at DESC);

CREATE TABLE IF NOT EXISTS subscription (
    id               BIGSERIAL PRIMARY KEY,
    name             VARCHAR(128) NOT NULL,
    source_id        BIGINT NOT NULL REFERENCES source(id),
    event_filter     JSONB NOT NULL DEFAULT '["*"]',
    target_url       VARCHAR(512) NOT NULL,
    signing_secret   VARCHAR(256) DEFAULT '',
    custom_headers   JSONB DEFAULT '{}',
    transform        JSONB,
    max_retries      INT DEFAULT 8,
    timeout_seconds  INT DEFAULT 30,
    rate_limit_rps   INT DEFAULT 100,
    active           BOOLEAN DEFAULT true,
    created_at       TIMESTAMPTZ DEFAULT now(),
    updated_at       TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sub_source
    ON subscription(source_id, active);

CREATE TABLE IF NOT EXISTS delivery (
    id               BIGSERIAL PRIMARY KEY,
    event_id         BIGINT NOT NULL REFERENCES event(id),
    subscription_id  BIGINT NOT NULL REFERENCES subscription(id),
    status           SMALLINT NOT NULL DEFAULT 0,
    attempt_count    INT DEFAULT 0,
    max_retries      INT NOT NULL,
    next_attempt_at  TIMESTAMPTZ,
    last_status_code INT,
    last_response    TEXT,
    last_error       TEXT,
    last_duration_ms INT,
    created_at       TIMESTAMPTZ DEFAULT now(),
    completed_at     TIMESTAMPTZ,

    UNIQUE(event_id, subscription_id)
);

CREATE INDEX IF NOT EXISTS idx_delivery_pending
    ON delivery(next_attempt_at NULLS FIRST)
    WHERE status IN (0, 3);

CREATE TABLE IF NOT EXISTS delivery_attempt (
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

CREATE INDEX IF NOT EXISTS idx_attempt_delivery
    ON delivery_attempt(delivery_id, attempt_number);
