package store

// ---------------------------------------------------------------------------
// Source queries
// ---------------------------------------------------------------------------

const queryCreateSource = `
INSERT INTO source (name, verify_type, verify_config, event_type_path, event_type_header,
                    idempotency_path, idempotency_header, description, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
RETURNING id, name, verify_type, verify_config, event_type_path, event_type_header,
          idempotency_path, idempotency_header, description, active, created_at, updated_at`

const queryGetSource = `
SELECT id, name, verify_type, verify_config, event_type_path, event_type_header,
       idempotency_path, idempotency_header, description, active, created_at, updated_at
FROM source WHERE id = $1`

const queryGetSourceByName = `
SELECT id, name, verify_type, verify_config, event_type_path, event_type_header,
       idempotency_path, idempotency_header, description, active, created_at, updated_at
FROM source WHERE name = $1`

const queryListSources = `
SELECT id, name, verify_type, verify_config, event_type_path, event_type_header,
       idempotency_path, idempotency_header, description, active, created_at, updated_at
FROM source ORDER BY id`

const queryUpdateSource = `
UPDATE source
SET name = $2, verify_type = $3, verify_config = $4, event_type_path = $5, event_type_header = $6,
    idempotency_path = $7, idempotency_header = $8, description = $9, active = $10, updated_at = now()
WHERE id = $1`

const queryDeleteSource = `DELETE FROM source WHERE id = $1`

// ---------------------------------------------------------------------------
// Event queries
// ---------------------------------------------------------------------------

const queryCreateEvent = `
INSERT INTO event (source_id, source_name, event_type, idempotency_key, headers, payload, raw_body, remote_addr, received_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
RETURNING id, source_id, source_name, event_type, idempotency_key, headers, payload, raw_body, remote_addr, received_at`

const queryGetEvent = `
SELECT id, source_id, source_name, event_type, idempotency_key, headers, payload, raw_body, remote_addr, received_at
FROM event WHERE id = $1`

const queryFindEventByIdempotencyKey = `
SELECT id, source_id, source_name, event_type, idempotency_key, headers, payload, raw_body, remote_addr, received_at
FROM event WHERE source_id = $1 AND idempotency_key = $2`

// queryListEventsBase is dynamically extended with WHERE clauses in the implementation.
const queryListEventsBase = `
SELECT id, source_id, source_name, event_type, idempotency_key, headers, payload, raw_body, remote_addr, received_at
FROM event`

const queryCountEventsBase = `SELECT count(*) FROM event`

// ---------------------------------------------------------------------------
// Subscription queries
// ---------------------------------------------------------------------------

const queryCreateSubscription = `
INSERT INTO subscription (name, source_id, event_filter, target_url, signing_secret, custom_headers,
                          transform, max_retries, timeout_seconds, rate_limit_rps, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), now())
RETURNING id, name, source_id, event_filter, target_url, signing_secret, custom_headers,
          transform, max_retries, timeout_seconds, rate_limit_rps, active, created_at, updated_at`

const queryGetSubscription = `
SELECT id, name, source_id, event_filter, target_url, signing_secret, custom_headers,
       transform, max_retries, timeout_seconds, rate_limit_rps, active, created_at, updated_at
FROM subscription WHERE id = $1`

const queryListSubscriptions = `
SELECT id, name, source_id, event_filter, target_url, signing_secret, custom_headers,
       transform, max_retries, timeout_seconds, rate_limit_rps, active, created_at, updated_at
FROM subscription WHERE source_id = $1 ORDER BY id`

const queryListActiveSubscriptions = `
SELECT id, name, source_id, event_filter, target_url, signing_secret, custom_headers,
       transform, max_retries, timeout_seconds, rate_limit_rps, active, created_at, updated_at
FROM subscription WHERE source_id = $1 AND active = true ORDER BY id`

const queryUpdateSubscription = `
UPDATE subscription
SET name = $2, source_id = $3, event_filter = $4, target_url = $5, signing_secret = $6,
    custom_headers = $7, transform = $8, max_retries = $9, timeout_seconds = $10,
    rate_limit_rps = $11, active = $12, updated_at = now()
WHERE id = $1`

const queryDeleteSubscription = `DELETE FROM subscription WHERE id = $1`

const queryGetSubscriptionStats = `
SELECT
    count(*)                                                             AS total,
    count(*) FILTER (WHERE status = 2)                                   AS success,
    count(*) FILTER (WHERE status = 3)                                   AS failed,
    count(*) FILTER (WHERE status = 4)                                   AS dead_letter,
    coalesce(avg(last_duration_ms) FILTER (WHERE status = 2), 0)         AS avg_latency_ms,
    coalesce(percentile_cont(0.99) WITHIN GROUP (ORDER BY last_duration_ms)
             FILTER (WHERE status = 2), 0)                               AS p99_latency_ms,
    CASE WHEN count(*) > 0
         THEN count(*) FILTER (WHERE status = 2)::float / count(*)::float
         ELSE 0 END                                                      AS success_rate
FROM delivery WHERE subscription_id = $1`

// ---------------------------------------------------------------------------
// Delivery queries
// ---------------------------------------------------------------------------

const queryCreateDelivery = `
INSERT INTO delivery (event_id, subscription_id, status, attempt_count, max_retries,
                      next_attempt_at, last_status_code, last_response, last_error,
                      last_duration_ms, created_at, completed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now(), $11)
RETURNING id, event_id, subscription_id, status, attempt_count, max_retries,
          next_attempt_at, last_status_code, last_response, last_error,
          last_duration_ms, created_at, completed_at`

const queryGetDelivery = `
SELECT id, event_id, subscription_id, status, attempt_count, max_retries,
       next_attempt_at, last_status_code, last_response, last_error,
       last_duration_ms, created_at, completed_at
FROM delivery WHERE id = $1`

const queryUpdateDelivery = `
UPDATE delivery
SET status = $2, attempt_count = $3, max_retries = $4, next_attempt_at = $5,
    last_status_code = $6, last_response = $7, last_error = $8,
    last_duration_ms = $9, completed_at = $10
WHERE id = $1`

const queryFetchPendingDeliveries = `
SELECT id, event_id, subscription_id, status, attempt_count, max_retries,
       next_attempt_at, last_status_code, last_response, last_error,
       last_duration_ms, created_at, completed_at
FROM delivery
WHERE status IN (0, 3) AND (next_attempt_at IS NULL OR next_attempt_at <= now())
ORDER BY next_attempt_at NULLS FIRST
LIMIT $1
FOR UPDATE SKIP LOCKED`

const queryListDeliveriesByEvent = `
SELECT id, event_id, subscription_id, status, attempt_count, max_retries,
       next_attempt_at, last_status_code, last_response, last_error,
       last_duration_ms, created_at, completed_at
FROM delivery WHERE event_id = $1 ORDER BY id`

// queryListDeadLettersBase is dynamically extended with optional filters.
const queryListDeadLettersBase = `
SELECT id, event_id, subscription_id, status, attempt_count, max_retries,
       next_attempt_at, last_status_code, last_response, last_error,
       last_duration_ms, created_at, completed_at
FROM delivery WHERE status = 4`

const queryCountDeadLettersBase = `SELECT count(*) FROM delivery WHERE status = 4`

const queryRetryDeadLetter = `
UPDATE delivery
SET status = 0, next_attempt_at = now()
WHERE id = $1 AND status = 4`

// ---------------------------------------------------------------------------
// DeliveryAttempt queries
// ---------------------------------------------------------------------------

const queryCreateDeliveryAttempt = `
INSERT INTO delivery_attempt (delivery_id, attempt_number, status_code, response_body, error, duration_ms, request_headers, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, delivery_id, attempt_number, status_code, response_body, error, duration_ms, request_headers, created_at`

const queryListDeliveryAttempts = `
SELECT id, delivery_id, attempt_number, status_code, response_body, error, duration_ms, request_headers, created_at
FROM delivery_attempt WHERE delivery_id = $1 ORDER BY attempt_number`

// ---------------------------------------------------------------------------
// Stats queries
// ---------------------------------------------------------------------------

const queryOverviewStats = `
SELECT
    (SELECT count(*) FROM event WHERE received_at >= date_trunc('day', now()))       AS events_today,
    (SELECT count(*) FROM delivery WHERE created_at >= date_trunc('day', now()))     AS deliveries_today,
    CASE WHEN (SELECT count(*) FROM delivery WHERE created_at >= date_trunc('day', now())) > 0
         THEN (SELECT count(*) FROM delivery WHERE created_at >= date_trunc('day', now()) AND status = 2)::float /
              (SELECT count(*) FROM delivery WHERE created_at >= date_trunc('day', now()))::float
         ELSE 0 END                                                                  AS success_rate,
    coalesce((SELECT avg(last_duration_ms) FROM delivery
              WHERE created_at >= date_trunc('day', now()) AND status = 2), 0)       AS avg_latency_ms,
    (SELECT count(*) FROM delivery WHERE status = 4)                                 AS dead_letters_pending,
    (SELECT count(*) FROM source WHERE active = true)                                AS active_sources,
    (SELECT count(*) FROM subscription WHERE active = true)                          AS active_subscriptions`

const queryThroughput = `
SELECT
    date_trunc($3, gs.t)                                                          AS timestamp,
    coalesce((SELECT count(*) FROM event
              WHERE received_at >= gs.t AND received_at < gs.t + $3::interval), 0) AS events,
    coalesce((SELECT count(*) FROM delivery
              WHERE created_at >= gs.t AND created_at < gs.t + $3::interval), 0)   AS deliveries,
    coalesce((SELECT count(*) FROM delivery
              WHERE created_at >= gs.t AND created_at < gs.t + $3::interval
              AND status = 2), 0)                                                  AS success,
    coalesce((SELECT count(*) FROM delivery
              WHERE created_at >= gs.t AND created_at < gs.t + $3::interval
              AND status IN (3,4)), 0)                                             AS failed
FROM generate_series($1::timestamptz, $2::timestamptz, $3::interval) AS gs(t)
ORDER BY gs.t`
