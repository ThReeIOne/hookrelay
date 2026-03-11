package delivery

import (
	"math"
	"math/rand"
	"time"

	"github.com/hookrelay/hookrelay/internal/metrics"
	"github.com/hookrelay/hookrelay/internal/store"
)

// Retry delay table (seconds).
// Attempt 1: 10s, 2: 30s, 3: 1m, 4: 5m, 5: 15m, 6: 1h, 7: 4h, 8: 12h
var retryDelays = []int{10, 30, 60, 300, 900, 3600, 14400, 43200}

// scheduleRetry sets the next retry time or moves to dead letter queue.
func scheduleRetry(d *store.Delivery) {
	if d.AttemptCount >= d.MaxRetries {
		d.Status = store.StatusDeadLetter
		metrics.DeadLettersTotal.Inc()
		now := time.Now()
		d.CompletedAt = &now
		return
	}

	d.Status = store.StatusFailed
	metrics.RetryTotal.Inc()
	idx := d.AttemptCount - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(retryDelays) {
		idx = len(retryDelays) - 1
	}
	delay := retryDelays[idx]
	// Add 10% random jitter to prevent retry storms
	jitter := float64(delay) * 0.1 * rand.Float64()
	next := time.Now().Add(time.Duration(math.Round(float64(delay)+jitter)) * time.Second)
	d.NextAttemptAt = &next
}

// truncate truncates a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
