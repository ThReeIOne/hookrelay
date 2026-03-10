package store

import "time"

// Delivery status constants.
const (
	StatusPending    = 0
	StatusDelivering = 1
	StatusSuccess    = 2
	StatusFailed     = 3
	StatusDeadLetter = 4
)

// Source represents a webhook source (e.g. GitHub, Stripe).
type Source struct {
	ID                int64          `json:"id"`
	Name              string         `json:"name"`
	VerifyType        string         `json:"verify_type"`
	VerifyConfig      map[string]any `json:"verify_config"`
	EventTypePath     string         `json:"event_type_path"`
	EventTypeHeader   string         `json:"event_type_header"`
	IdempotencyPath   string         `json:"idempotency_path"`
	IdempotencyHeader string         `json:"idempotency_header"`
	Description       string         `json:"description"`
	Active            bool           `json:"active"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// Event represents a received webhook event.
type Event struct {
	ID             int64             `json:"id"`
	SourceID       int64             `json:"source_id"`
	SourceName     string            `json:"source_name"`
	EventType      string            `json:"event_type"`
	IdempotencyKey *string           `json:"idempotency_key"` // pointer for SQL NULL
	Headers        map[string]string `json:"headers"`
	Payload        map[string]any    `json:"payload"`
	RawBody        []byte            `json:"-"`
	RemoteAddr     string            `json:"remote_addr"`
	ReceivedAt     time.Time         `json:"received_at"`
}

// Subscription represents a webhook delivery target.
type Subscription struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	SourceID       int64             `json:"source_id"`
	EventFilter    []string          `json:"event_filter"`
	TargetURL      string            `json:"target_url"`
	SigningSecret  string            `json:"signing_secret,omitempty"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	Transform      map[string]any    `json:"transform,omitempty"`
	MaxRetries     int               `json:"max_retries"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	RateLimitRPS   int               `json:"rate_limit_rps"`
	Active         bool              `json:"active"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Delivery represents a single delivery attempt lifecycle for an event to a subscription.
type Delivery struct {
	ID             int64      `json:"id"`
	EventID        int64      `json:"event_id"`
	SubscriptionID int64      `json:"subscription_id"`
	Status         int        `json:"status"`
	AttemptCount   int        `json:"attempt_count"`
	MaxRetries     int        `json:"max_retries"`
	NextAttemptAt  *time.Time `json:"next_attempt_at"`
	LastStatusCode int        `json:"last_status_code"`
	LastResponse   string     `json:"last_response,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	LastDurationMs int        `json:"last_duration_ms"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

// DeliveryAttempt records a single HTTP attempt within a Delivery.
type DeliveryAttempt struct {
	ID             int64             `json:"id"`
	DeliveryID     int64             `json:"delivery_id"`
	AttemptNumber  int               `json:"attempt_number"`
	StatusCode     int               `json:"status_code"`
	ResponseBody   string            `json:"response_body,omitempty"`
	Error          string            `json:"error,omitempty"`
	DurationMs     int               `json:"duration_ms"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// EventFilter is used to query events with optional filtering and pagination.
type EventFilter struct {
	SourceName string
	EventType  string
	Start      *time.Time
	End        *time.Time
	Page       int
	PageSize   int
}

// DeliveryFilter is used to query deliveries with optional filtering and pagination.
type DeliveryFilter struct {
	SubscriptionID int64
	Status         *int
	Page           int
	PageSize       int
}

// SubscriptionStats holds aggregate delivery statistics for a subscription.
type SubscriptionStats struct {
	Total        int64   `json:"total"`
	Success      int64   `json:"success"`
	Failed       int64   `json:"failed"`
	DeadLetter   int64   `json:"dead_letter"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P99LatencyMs float64 `json:"p99_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
}

// OverviewStats holds dashboard-level aggregate statistics.
type OverviewStats struct {
	EventsToday         int64   `json:"events_today"`
	DeliveriesToday     int64   `json:"deliveries_today"`
	SuccessRate         float64 `json:"success_rate"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
	DeadLettersPending  int64   `json:"dead_letters_pending"`
	ActiveSources       int64   `json:"active_sources"`
	ActiveSubscriptions int64   `json:"active_subscriptions"`
}

// ThroughputPoint is a single data point for time-series throughput charts.
type ThroughputPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Events     int64     `json:"events"`
	Deliveries int64     `json:"deliveries"`
	Success    int64     `json:"success"`
	Failed     int64     `json:"failed"`
}
