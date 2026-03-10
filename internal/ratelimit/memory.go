package ratelimit

import (
	"context"
	"sync"
	"time"
)

// window tracks request timestamps for a single subscription.
type window struct {
	mu         sync.Mutex
	timestamps []int64 // unix microseconds
}

// MemoryRateLimiter is an in-memory sliding window rate limiter.
// It uses a sync.Map to store per-subscription windows, where each window
// tracks request timestamps within a 1-second sliding window.
type MemoryRateLimiter struct {
	windows sync.Map // map[int64]*window
}

// NewMemoryRateLimiter creates a new in-memory rate limiter.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{}
}

// Allow checks whether a request for the given subscription is allowed
// under the specified requests-per-second limit.
func (r *MemoryRateLimiter) Allow(_ context.Context, subscriptionID int64, rps int) bool {
	if rps <= 0 {
		return false
	}

	// Load or create the window for this subscription.
	val, _ := r.windows.LoadOrStore(subscriptionID, &window{})
	w := val.(*window)

	now := time.Now().UnixMicro()
	cutoff := now - int64(time.Second/time.Microsecond) // 1 second ago in microseconds

	w.mu.Lock()
	defer w.mu.Unlock()

	// Remove expired timestamps (older than 1 second).
	start := 0
	for start < len(w.timestamps) && w.timestamps[start] < cutoff {
		start++
	}
	w.timestamps = w.timestamps[start:]

	// Check if we're at the limit.
	if len(w.timestamps) >= rps {
		return false
	}

	// Record this request.
	w.timestamps = append(w.timestamps, now)
	return true
}
