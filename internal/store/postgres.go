package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store using pgxpool.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// Compile-time check that PostgresStore satisfies the Store interface.
var _ Store = (*PostgresStore)(nil)

// NewPostgresStore creates a new PostgresStore with the given connection parameters.
func NewPostgresStore(ctx context.Context, dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing dsn: %w", err)
	}

	cfg.MaxConns = int32(maxOpen)
	cfg.MinConns = int32(maxIdle)
	cfg.MaxConnLifetime = maxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

// marshalJSON marshals v to JSON bytes, returning nil for nil input.
func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// scanJSON scans raw JSONB bytes into dest.
func scanJSON(raw []byte, dest any) error {
	if raw == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

// ---------------------------------------------------------------------------
// Source
// ---------------------------------------------------------------------------

func (s *PostgresStore) CreateSource(ctx context.Context, src *Source) (*Source, error) {
	verifyConfigJSON, err := marshalJSON(src.VerifyConfig)
	if err != nil {
		return nil, fmt.Errorf("marshalling verify_config: %w", err)
	}

	out := &Source{}
	var verifyConfigRaw []byte

	err = s.pool.QueryRow(ctx, queryCreateSource,
		src.Name, src.VerifyType, verifyConfigJSON, src.EventTypePath, src.EventTypeHeader,
		src.IdempotencyPath, src.IdempotencyHeader, src.Description, src.Active,
	).Scan(
		&out.ID, &out.Name, &out.VerifyType, &verifyConfigRaw,
		&out.EventTypePath, &out.EventTypeHeader,
		&out.IdempotencyPath, &out.IdempotencyHeader,
		&out.Description, &out.Active, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating source: %w", err)
	}

	if err := scanJSON(verifyConfigRaw, &out.VerifyConfig); err != nil {
		return nil, fmt.Errorf("scanning verify_config: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) GetSource(ctx context.Context, id int64) (*Source, error) {
	return s.scanSource(s.pool.QueryRow(ctx, queryGetSource, id))
}

func (s *PostgresStore) GetSourceByName(ctx context.Context, name string) (*Source, error) {
	return s.scanSource(s.pool.QueryRow(ctx, queryGetSourceByName, name))
}

func (s *PostgresStore) ListSources(ctx context.Context) ([]*Source, error) {
	rows, err := s.pool.Query(ctx, queryListSources)
	if err != nil {
		return nil, fmt.Errorf("listing sources: %w", err)
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		src, err := s.scanSourceFromRow(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func (s *PostgresStore) UpdateSource(ctx context.Context, src *Source) error {
	verifyConfigJSON, err := marshalJSON(src.VerifyConfig)
	if err != nil {
		return fmt.Errorf("marshalling verify_config: %w", err)
	}

	_, err = s.pool.Exec(ctx, queryUpdateSource,
		src.ID, src.Name, src.VerifyType, verifyConfigJSON, src.EventTypePath, src.EventTypeHeader,
		src.IdempotencyPath, src.IdempotencyHeader, src.Description, src.Active,
	)
	if err != nil {
		return fmt.Errorf("updating source: %w", err)
	}
	return nil
}

func (s *PostgresStore) DeleteSource(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, queryDeleteSource, id)
	if err != nil {
		return fmt.Errorf("deleting source: %w", err)
	}
	return nil
}

func (s *PostgresStore) scanSource(row pgx.Row) (*Source, error) {
	out := &Source{}
	var verifyConfigRaw []byte

	err := row.Scan(
		&out.ID, &out.Name, &out.VerifyType, &verifyConfigRaw,
		&out.EventTypePath, &out.EventTypeHeader,
		&out.IdempotencyPath, &out.IdempotencyHeader,
		&out.Description, &out.Active, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}

	if err := scanJSON(verifyConfigRaw, &out.VerifyConfig); err != nil {
		return nil, fmt.Errorf("scanning verify_config: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) scanSourceFromRow(rows pgx.Rows) (*Source, error) {
	out := &Source{}
	var verifyConfigRaw []byte

	err := rows.Scan(
		&out.ID, &out.Name, &out.VerifyType, &verifyConfigRaw,
		&out.EventTypePath, &out.EventTypeHeader,
		&out.IdempotencyPath, &out.IdempotencyHeader,
		&out.Description, &out.Active, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning source row: %w", err)
	}

	if err := scanJSON(verifyConfigRaw, &out.VerifyConfig); err != nil {
		return nil, fmt.Errorf("scanning verify_config: %w", err)
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Event
// ---------------------------------------------------------------------------

func (s *PostgresStore) CreateEvent(ctx context.Context, e *Event) (*Event, error) {
	headersJSON, err := marshalJSON(e.Headers)
	if err != nil {
		return nil, fmt.Errorf("marshalling headers: %w", err)
	}
	payloadJSON, err := marshalJSON(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling payload: %w", err)
	}

	out := &Event{}
	var headersRaw, payloadRaw []byte

	err = s.pool.QueryRow(ctx, queryCreateEvent,
		e.SourceID, e.SourceName, e.EventType, e.IdempotencyKey,
		headersJSON, payloadJSON, e.RawBody, e.RemoteAddr,
	).Scan(
		&out.ID, &out.SourceID, &out.SourceName, &out.EventType, &out.IdempotencyKey,
		&headersRaw, &payloadRaw, &out.RawBody, &out.RemoteAddr, &out.ReceivedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating event: %w", err)
	}

	if err := scanJSON(headersRaw, &out.Headers); err != nil {
		return nil, fmt.Errorf("scanning headers: %w", err)
	}
	if err := scanJSON(payloadRaw, &out.Payload); err != nil {
		return nil, fmt.Errorf("scanning payload: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) GetEvent(ctx context.Context, id int64) (*Event, error) {
	return s.scanEvent(s.pool.QueryRow(ctx, queryGetEvent, id))
}

func (s *PostgresStore) FindEventByIdempotencyKey(ctx context.Context, sourceID int64, key string) (*Event, error) {
	return s.scanEvent(s.pool.QueryRow(ctx, queryFindEventByIdempotencyKey, sourceID, key))
}

func (s *PostgresStore) ListEvents(ctx context.Context, filter EventFilter) ([]*Event, int64, error) {
	// Build dynamic WHERE clause.
	conditions := []string{}
	args := []any{}
	argIdx := 1

	if filter.SourceName != "" {
		conditions = append(conditions, fmt.Sprintf("source_name = $%d", argIdx))
		args = append(args, filter.SourceName)
		argIdx++
	}
	if filter.EventType != "" {
		conditions = append(conditions, fmt.Sprintf("event_type = $%d", argIdx))
		args = append(args, filter.EventType)
		argIdx++
	}
	if filter.Start != nil {
		conditions = append(conditions, fmt.Sprintf("received_at >= $%d", argIdx))
		args = append(args, *filter.Start)
		argIdx++
	}
	if filter.End != nil {
		conditions = append(conditions, fmt.Sprintf("received_at <= $%d", argIdx))
		args = append(args, *filter.End)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total.
	var total int64
	countQuery := queryCountEventsBase + where
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting events: %w", err)
	}

	// Pagination defaults.
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	listQuery := queryListEventsBase + where +
		fmt.Sprintf(" ORDER BY received_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		ev, err := s.scanEventFromRow(rows)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, ev)
	}
	return events, total, rows.Err()
}

func (s *PostgresStore) scanEvent(row pgx.Row) (*Event, error) {
	out := &Event{}
	var headersRaw, payloadRaw []byte

	err := row.Scan(
		&out.ID, &out.SourceID, &out.SourceName, &out.EventType, &out.IdempotencyKey,
		&headersRaw, &payloadRaw, &out.RawBody, &out.RemoteAddr, &out.ReceivedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning event: %w", err)
	}

	if err := scanJSON(headersRaw, &out.Headers); err != nil {
		return nil, fmt.Errorf("scanning headers: %w", err)
	}
	if err := scanJSON(payloadRaw, &out.Payload); err != nil {
		return nil, fmt.Errorf("scanning payload: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) scanEventFromRow(rows pgx.Rows) (*Event, error) {
	out := &Event{}
	var headersRaw, payloadRaw []byte

	err := rows.Scan(
		&out.ID, &out.SourceID, &out.SourceName, &out.EventType, &out.IdempotencyKey,
		&headersRaw, &payloadRaw, &out.RawBody, &out.RemoteAddr, &out.ReceivedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning event row: %w", err)
	}

	if err := scanJSON(headersRaw, &out.Headers); err != nil {
		return nil, fmt.Errorf("scanning headers: %w", err)
	}
	if err := scanJSON(payloadRaw, &out.Payload); err != nil {
		return nil, fmt.Errorf("scanning payload: %w", err)
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Subscription
// ---------------------------------------------------------------------------

func (s *PostgresStore) CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error) {
	eventFilterJSON, err := marshalJSON(sub.EventFilter)
	if err != nil {
		return nil, fmt.Errorf("marshalling event_filter: %w", err)
	}
	customHeadersJSON, err := marshalJSON(sub.CustomHeaders)
	if err != nil {
		return nil, fmt.Errorf("marshalling custom_headers: %w", err)
	}
	transformJSON, err := marshalJSON(sub.Transform)
	if err != nil {
		return nil, fmt.Errorf("marshalling transform: %w", err)
	}

	out := &Subscription{}
	var eventFilterRaw, customHeadersRaw, transformRaw []byte

	err = s.pool.QueryRow(ctx, queryCreateSubscription,
		sub.Name, sub.SourceID, eventFilterJSON, sub.TargetURL, sub.SigningSecret,
		customHeadersJSON, transformJSON, sub.MaxRetries, sub.TimeoutSeconds,
		sub.RateLimitRPS, sub.Active,
	).Scan(
		&out.ID, &out.Name, &out.SourceID, &eventFilterRaw, &out.TargetURL, &out.SigningSecret,
		&customHeadersRaw, &transformRaw, &out.MaxRetries, &out.TimeoutSeconds,
		&out.RateLimitRPS, &out.Active, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating subscription: %w", err)
	}

	if err := scanJSON(eventFilterRaw, &out.EventFilter); err != nil {
		return nil, fmt.Errorf("scanning event_filter: %w", err)
	}
	if err := scanJSON(customHeadersRaw, &out.CustomHeaders); err != nil {
		return nil, fmt.Errorf("scanning custom_headers: %w", err)
	}
	if err := scanJSON(transformRaw, &out.Transform); err != nil {
		return nil, fmt.Errorf("scanning transform: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) GetSubscription(ctx context.Context, id int64) (*Subscription, error) {
	return s.scanSubscription(s.pool.QueryRow(ctx, queryGetSubscription, id))
}

func (s *PostgresStore) ListSubscriptions(ctx context.Context, sourceID int64) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx, queryListSubscriptions, sourceID)
	if err != nil {
		return nil, fmt.Errorf("listing subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub, err := s.scanSubscriptionFromRow(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (s *PostgresStore) ListActiveSubscriptions(ctx context.Context, sourceID int64) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx, queryListActiveSubscriptions, sourceID)
	if err != nil {
		return nil, fmt.Errorf("listing active subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		sub, err := s.scanSubscriptionFromRow(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (s *PostgresStore) UpdateSubscription(ctx context.Context, sub *Subscription) error {
	eventFilterJSON, err := marshalJSON(sub.EventFilter)
	if err != nil {
		return fmt.Errorf("marshalling event_filter: %w", err)
	}
	customHeadersJSON, err := marshalJSON(sub.CustomHeaders)
	if err != nil {
		return fmt.Errorf("marshalling custom_headers: %w", err)
	}
	transformJSON, err := marshalJSON(sub.Transform)
	if err != nil {
		return fmt.Errorf("marshalling transform: %w", err)
	}

	_, err = s.pool.Exec(ctx, queryUpdateSubscription,
		sub.ID, sub.Name, sub.SourceID, eventFilterJSON, sub.TargetURL, sub.SigningSecret,
		customHeadersJSON, transformJSON, sub.MaxRetries, sub.TimeoutSeconds,
		sub.RateLimitRPS, sub.Active,
	)
	if err != nil {
		return fmt.Errorf("updating subscription: %w", err)
	}
	return nil
}

func (s *PostgresStore) DeleteSubscription(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, queryDeleteSubscription, id)
	if err != nil {
		return fmt.Errorf("deleting subscription: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetSubscriptionStats(ctx context.Context, id int64) (*SubscriptionStats, error) {
	out := &SubscriptionStats{}
	err := s.pool.QueryRow(ctx, queryGetSubscriptionStats, id).Scan(
		&out.Total, &out.Success, &out.Failed, &out.DeadLetter,
		&out.AvgLatencyMs, &out.P99LatencyMs, &out.SuccessRate,
	)
	if err != nil {
		return nil, fmt.Errorf("getting subscription stats: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) scanSubscription(row pgx.Row) (*Subscription, error) {
	out := &Subscription{}
	var eventFilterRaw, customHeadersRaw, transformRaw []byte

	err := row.Scan(
		&out.ID, &out.Name, &out.SourceID, &eventFilterRaw, &out.TargetURL, &out.SigningSecret,
		&customHeadersRaw, &transformRaw, &out.MaxRetries, &out.TimeoutSeconds,
		&out.RateLimitRPS, &out.Active, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning subscription: %w", err)
	}

	if err := scanJSON(eventFilterRaw, &out.EventFilter); err != nil {
		return nil, fmt.Errorf("scanning event_filter: %w", err)
	}
	if err := scanJSON(customHeadersRaw, &out.CustomHeaders); err != nil {
		return nil, fmt.Errorf("scanning custom_headers: %w", err)
	}
	if err := scanJSON(transformRaw, &out.Transform); err != nil {
		return nil, fmt.Errorf("scanning transform: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) scanSubscriptionFromRow(rows pgx.Rows) (*Subscription, error) {
	out := &Subscription{}
	var eventFilterRaw, customHeadersRaw, transformRaw []byte

	err := rows.Scan(
		&out.ID, &out.Name, &out.SourceID, &eventFilterRaw, &out.TargetURL, &out.SigningSecret,
		&customHeadersRaw, &transformRaw, &out.MaxRetries, &out.TimeoutSeconds,
		&out.RateLimitRPS, &out.Active, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning subscription row: %w", err)
	}

	if err := scanJSON(eventFilterRaw, &out.EventFilter); err != nil {
		return nil, fmt.Errorf("scanning event_filter: %w", err)
	}
	if err := scanJSON(customHeadersRaw, &out.CustomHeaders); err != nil {
		return nil, fmt.Errorf("scanning custom_headers: %w", err)
	}
	if err := scanJSON(transformRaw, &out.Transform); err != nil {
		return nil, fmt.Errorf("scanning transform: %w", err)
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Delivery
// ---------------------------------------------------------------------------

func (s *PostgresStore) CreateDelivery(ctx context.Context, d *Delivery) (*Delivery, error) {
	out := &Delivery{}

	err := s.pool.QueryRow(ctx, queryCreateDelivery,
		d.EventID, d.SubscriptionID, d.Status, d.AttemptCount, d.MaxRetries,
		d.NextAttemptAt, d.LastStatusCode, d.LastResponse, d.LastError,
		d.LastDurationMs, d.CompletedAt,
	).Scan(
		&out.ID, &out.EventID, &out.SubscriptionID, &out.Status, &out.AttemptCount, &out.MaxRetries,
		&out.NextAttemptAt, &out.LastStatusCode, &out.LastResponse, &out.LastError,
		&out.LastDurationMs, &out.CreatedAt, &out.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating delivery: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) GetDelivery(ctx context.Context, id int64) (*Delivery, error) {
	return s.scanDelivery(s.pool.QueryRow(ctx, queryGetDelivery, id))
}

func (s *PostgresStore) UpdateDelivery(ctx context.Context, d *Delivery) error {
	_, err := s.pool.Exec(ctx, queryUpdateDelivery,
		d.ID, d.Status, d.AttemptCount, d.MaxRetries, d.NextAttemptAt,
		d.LastStatusCode, d.LastResponse, d.LastError,
		d.LastDurationMs, d.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("updating delivery: %w", err)
	}
	return nil
}

func (s *PostgresStore) FetchPendingDeliveries(ctx context.Context, limit int) ([]*Delivery, error) {
	rows, err := s.pool.Query(ctx, queryFetchPendingDeliveries, limit)
	if err != nil {
		return nil, fmt.Errorf("fetching pending deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		d, err := s.scanDeliveryFromRow(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

func (s *PostgresStore) ListDeliveriesByEvent(ctx context.Context, eventID int64) ([]*Delivery, error) {
	rows, err := s.pool.Query(ctx, queryListDeliveriesByEvent, eventID)
	if err != nil {
		return nil, fmt.Errorf("listing deliveries by event: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		d, err := s.scanDeliveryFromRow(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

func (s *PostgresStore) ListDeadLetters(ctx context.Context, filter DeliveryFilter) ([]*Delivery, int64, error) {
	conditions := []string{}
	args := []any{}
	argIdx := 1

	if filter.SubscriptionID > 0 {
		conditions = append(conditions, fmt.Sprintf("subscription_id = $%d", argIdx))
		args = append(args, filter.SubscriptionID)
		argIdx++
	}

	extra := ""
	if len(conditions) > 0 {
		extra = " AND " + strings.Join(conditions, " AND ")
	}

	// Count total dead letters matching filter.
	var total int64
	countQuery := queryCountDeadLettersBase + extra
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting dead letters: %w", err)
	}

	// Pagination defaults.
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	listQuery := queryListDeadLettersBase + extra +
		fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing dead letters: %w", err)
	}
	defer rows.Close()

	var deliveries []*Delivery
	for rows.Next() {
		d, err := s.scanDeliveryFromRow(rows)
		if err != nil {
			return nil, 0, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, total, rows.Err()
}

func (s *PostgresStore) RetryDeadLetter(ctx context.Context, deliveryID int64) error {
	tag, err := s.pool.Exec(ctx, queryRetryDeadLetter, deliveryID)
	if err != nil {
		return fmt.Errorf("retrying dead letter: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delivery %d is not in dead-letter status", deliveryID)
	}
	return nil
}

func (s *PostgresStore) BatchRetryDeadLetters(ctx context.Context, deliveryIDs []int64) error {
	if len(deliveryIDs) == 0 {
		return nil
	}

	// Build a parameterised IN clause: $1, $2, $3 ...
	placeholders := make([]string, len(deliveryIDs))
	args := make([]any, len(deliveryIDs))
	for i, id := range deliveryIDs {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		"UPDATE delivery SET status = 0, next_attempt_at = now() WHERE id IN (%s) AND status = 4",
		strings.Join(placeholders, ", "),
	)

	_, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("batch retrying dead letters: %w", err)
	}
	return nil
}

func (s *PostgresStore) scanDelivery(row pgx.Row) (*Delivery, error) {
	out := &Delivery{}
	err := row.Scan(
		&out.ID, &out.EventID, &out.SubscriptionID, &out.Status, &out.AttemptCount, &out.MaxRetries,
		&out.NextAttemptAt, &out.LastStatusCode, &out.LastResponse, &out.LastError,
		&out.LastDurationMs, &out.CreatedAt, &out.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning delivery: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) scanDeliveryFromRow(rows pgx.Rows) (*Delivery, error) {
	out := &Delivery{}
	err := rows.Scan(
		&out.ID, &out.EventID, &out.SubscriptionID, &out.Status, &out.AttemptCount, &out.MaxRetries,
		&out.NextAttemptAt, &out.LastStatusCode, &out.LastResponse, &out.LastError,
		&out.LastDurationMs, &out.CreatedAt, &out.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning delivery row: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// DeliveryAttempt
// ---------------------------------------------------------------------------

func (s *PostgresStore) CreateDeliveryAttempt(ctx context.Context, a *DeliveryAttempt) (*DeliveryAttempt, error) {
	requestHeadersJSON, err := marshalJSON(a.RequestHeaders)
	if err != nil {
		return nil, fmt.Errorf("marshalling request_headers: %w", err)
	}

	out := &DeliveryAttempt{}
	var requestHeadersRaw []byte

	err = s.pool.QueryRow(ctx, queryCreateDeliveryAttempt,
		a.DeliveryID, a.AttemptNumber, a.StatusCode, a.ResponseBody, a.Error,
		a.DurationMs, requestHeadersJSON,
	).Scan(
		&out.ID, &out.DeliveryID, &out.AttemptNumber, &out.StatusCode,
		&out.ResponseBody, &out.Error, &out.DurationMs, &requestHeadersRaw, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating delivery attempt: %w", err)
	}

	if err := scanJSON(requestHeadersRaw, &out.RequestHeaders); err != nil {
		return nil, fmt.Errorf("scanning request_headers: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) ListDeliveryAttempts(ctx context.Context, deliveryID int64) ([]*DeliveryAttempt, error) {
	rows, err := s.pool.Query(ctx, queryListDeliveryAttempts, deliveryID)
	if err != nil {
		return nil, fmt.Errorf("listing delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*DeliveryAttempt
	for rows.Next() {
		out := &DeliveryAttempt{}
		var requestHeadersRaw []byte

		err := rows.Scan(
			&out.ID, &out.DeliveryID, &out.AttemptNumber, &out.StatusCode,
			&out.ResponseBody, &out.Error, &out.DurationMs, &requestHeadersRaw, &out.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning delivery attempt row: %w", err)
		}

		if err := scanJSON(requestHeadersRaw, &out.RequestHeaders); err != nil {
			return nil, fmt.Errorf("scanning request_headers: %w", err)
		}

		attempts = append(attempts, out)
	}
	return attempts, rows.Err()
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func (s *PostgresStore) GetOverviewStats(ctx context.Context) (*OverviewStats, error) {
	out := &OverviewStats{}
	err := s.pool.QueryRow(ctx, queryOverviewStats).Scan(
		&out.EventsToday, &out.DeliveriesToday, &out.SuccessRate, &out.AvgLatencyMs,
		&out.DeadLettersPending, &out.ActiveSources, &out.ActiveSubscriptions,
	)
	if err != nil {
		return nil, fmt.Errorf("getting overview stats: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) GetThroughput(ctx context.Context, start, end time.Time, granularity string) ([]*ThroughputPoint, error) {
	rows, err := s.pool.Query(ctx, queryThroughput, start, end, granularity)
	if err != nil {
		return nil, fmt.Errorf("getting throughput: %w", err)
	}
	defer rows.Close()

	var points []*ThroughputPoint
	for rows.Next() {
		p := &ThroughputPoint{}
		if err := rows.Scan(&p.Timestamp, &p.Events, &p.Deliveries, &p.Success, &p.Failed); err != nil {
			return nil, fmt.Errorf("scanning throughput point: %w", err)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
