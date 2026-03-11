package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/hookrelay/hookrelay/internal/metrics"
	"github.com/hookrelay/hookrelay/internal/ratelimit"
	"github.com/hookrelay/hookrelay/internal/store"
	"github.com/hookrelay/hookrelay/internal/transform"
)

// Engine polls for pending deliveries and delivers them.
type Engine struct {
	store        store.Store
	rateLimiter  ratelimit.RateLimiter
	httpClient   *http.Client
	workers      int
	pollInterval time.Duration
	batchSize    int
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewEngine creates a delivery engine.
func NewEngine(s store.Store, rl ratelimit.RateLimiter, workers int, pollInterval time.Duration, batchSize int) *Engine {
	return &Engine{
		store:        s,
		rateLimiter:  rl,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		workers:      workers,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		stopCh:       make(chan struct{}),
	}
}

// Start launches worker goroutines.
func (e *Engine) Start(ctx context.Context) {
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go func(id int) {
			defer e.wg.Done()
			e.pollLoop(ctx, id)
		}(i)
	}
}

// Stop signals workers to stop and waits for them to finish.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

func (e *Engine) pollLoop(ctx context.Context, workerID int) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.poll(ctx)
		}
	}
}

func (e *Engine) poll(ctx context.Context) {
	deliveries, err := e.store.FetchPendingDeliveries(ctx, e.batchSize)
	if err != nil {
		slog.Error("fetch pending deliveries failed", "error", err)
		return
	}

	if len(deliveries) == 0 {
		return
	}

	metrics.DeliveryQueueDepth.Set(float64(len(deliveries)))

	var wg sync.WaitGroup
	for _, d := range deliveries {
		wg.Add(1)
		go func(d *store.Delivery) {
			defer wg.Done()
			e.deliver(ctx, d)
		}(d)
	}
	wg.Wait()
}

func (e *Engine) deliver(ctx context.Context, d *store.Delivery) {
	event, err := e.store.GetEvent(ctx, d.EventID)
	if err != nil {
		slog.Error("get event failed", "event_id", d.EventID, "error", err)
		return
	}

	sub, err := e.store.GetSubscription(ctx, d.SubscriptionID)
	if err != nil {
		slog.Error("get subscription failed", "sub_id", d.SubscriptionID, "error", err)
		return
	}

	// Rate limit check
	if !e.rateLimiter.Allow(ctx, d.SubscriptionID, sub.RateLimitRPS) {
		metrics.RateLimitedTotal.Inc()
		next := time.Now().Add(time.Second)
		d.NextAttemptAt = &next
		if err := e.store.UpdateDelivery(ctx, d); err != nil {
			slog.Error("update delivery after rate limit failed", "delivery_id", d.ID, "error", err)
		}
		return
	}

	// Transform payload
	payload := event.Payload
	if sub.Transform != nil {
		t, tErr := transform.GetTransformer(sub.Transform)
		if tErr != nil {
			slog.Error("get transformer failed", "sub_id", sub.ID, "error", tErr)
		} else {
			transformed, tErr := t.Transform(payload)
			if tErr != nil {
				slog.Error("transform payload failed", "sub_id", sub.ID, "error", tErr)
			} else {
				payload = transformed
			}
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("marshal payload failed", "delivery_id", d.ID, "error", err)
		return
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	headers := map[string]string{
		"Content-Type":          "application/json",
		"User-Agent":            "HookRelay/1.0",
		"X-HookRelay-Event-ID":  fmt.Sprintf("%d", event.ID),
		"X-HookRelay-Event":     event.EventType,
		"X-HookRelay-Timestamp": timestamp,
		"X-HookRelay-Attempt":   fmt.Sprintf("%d", d.AttemptCount+1),
	}

	// Merge custom headers
	for k, v := range sub.CustomHeaders {
		headers[k] = v
	}

	// Sign outbound request
	if sub.SigningSecret != "" {
		headers["X-HookRelay-Signature"] = sign(sub.SigningSecret, timestamp, bodyBytes)
	}

	// Create per-delivery context with subscription timeout
	timeout := time.Duration(sub.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deliverCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Send HTTP request
	start := time.Now()
	req, err := http.NewRequestWithContext(deliverCtx, "POST", sub.TargetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		slog.Error("create request failed", "delivery_id", d.ID, "error", err)
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := e.httpClient.Do(req)
	durationMs := int(time.Since(start).Milliseconds())
	metrics.DeliveryDurationMs.Observe(float64(durationMs))

	// Record attempt
	attempt := &store.DeliveryAttempt{
		DeliveryID:    d.ID,
		AttemptNumber: d.AttemptCount + 1,
		DurationMs:    durationMs,
	}

	d.AttemptCount++
	d.LastDurationMs = durationMs

	if err != nil {
		attempt.Error = err.Error()
		d.LastError = truncate(err.Error(), 2000)
		metrics.DeliveriesTotal.WithLabelValues("error").Inc()
		scheduleRetry(d)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

		attempt.StatusCode = resp.StatusCode
		attempt.ResponseBody = string(body)
		d.LastStatusCode = resp.StatusCode
		d.LastResponse = truncate(string(body), 2000)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			d.Status = store.StatusSuccess
			metrics.DeliveriesTotal.WithLabelValues("success").Inc()
			now := time.Now()
			d.CompletedAt = &now
		} else {
			metrics.DeliveriesTotal.WithLabelValues("failed").Inc()
			scheduleRetry(d)
		}
	}

	if _, err := e.store.CreateDeliveryAttempt(ctx, attempt); err != nil {
		slog.Error("create delivery attempt failed", "delivery_id", d.ID, "error", err)
	}
	if err := e.store.UpdateDelivery(ctx, d); err != nil {
		slog.Error("update delivery failed", "delivery_id", d.ID, "error", err)
	}
}
