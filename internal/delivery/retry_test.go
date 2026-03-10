package delivery

import (
	"testing"
	"time"

	"github.com/hookrelay/hookrelay/internal/store"
)

// ---------------------------------------------------------------------------
// scheduleRetry
// ---------------------------------------------------------------------------

func TestScheduleRetry_ExhaustedRetries(t *testing.T) {
	tests := []struct {
		name         string
		attemptCount int
		maxRetries   int
	}{
		{"equal", 3, 3},
		{"exceeded", 5, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &store.Delivery{
				AttemptCount: tt.attemptCount,
				MaxRetries:   tt.maxRetries,
			}
			before := time.Now()
			scheduleRetry(d)

			if d.Status != store.StatusDeadLetter {
				t.Errorf("expected status %d (StatusDeadLetter), got %d", store.StatusDeadLetter, d.Status)
			}
			if d.CompletedAt == nil {
				t.Fatal("expected CompletedAt to be set, got nil")
			}
			if d.CompletedAt.Before(before) {
				t.Error("CompletedAt is before the call to scheduleRetry")
			}
		})
	}
}

func TestScheduleRetry_HasRetriesRemaining(t *testing.T) {
	tests := []struct {
		name         string
		attemptCount int
		maxRetries   int
		expectedIdx  int // index into retryDelays for the base delay
	}{
		{"first retry", 1, 5, 0},
		{"second retry", 2, 5, 1},
		{"third retry", 3, 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &store.Delivery{
				AttemptCount: tt.attemptCount,
				MaxRetries:   tt.maxRetries,
			}
			before := time.Now()
			scheduleRetry(d)

			if d.Status != store.StatusFailed {
				t.Errorf("expected status %d (StatusFailed), got %d", store.StatusFailed, d.Status)
			}
			if d.NextAttemptAt == nil {
				t.Fatal("expected NextAttemptAt to be set, got nil")
			}

			baseDelay := retryDelays[tt.expectedIdx]
			// The delay should be between baseDelay and baseDelay * 1.1 (10% jitter).
			minNext := before.Add(time.Duration(baseDelay) * time.Second)
			maxNext := before.Add(time.Duration(float64(baseDelay)*1.1+1) * time.Second)

			if d.NextAttemptAt.Before(minNext) {
				t.Errorf("NextAttemptAt %v is before minimum expected %v", d.NextAttemptAt, minNext)
			}
			if d.NextAttemptAt.After(maxNext) {
				t.Errorf("NextAttemptAt %v is after maximum expected %v", d.NextAttemptAt, maxNext)
			}
		})
	}
}

func TestScheduleRetry_ExponentialBackoff(t *testing.T) {
	// Verify that delay increases with attempt count.
	var delays []time.Duration

	for attempt := 1; attempt <= 5; attempt++ {
		d := &store.Delivery{
			AttemptCount: attempt,
			MaxRetries:   10,
		}
		before := time.Now()
		scheduleRetry(d)
		delay := d.NextAttemptAt.Sub(before)
		delays = append(delays, delay)
	}

	for i := 1; i < len(delays); i++ {
		if delays[i] <= delays[i-1] {
			t.Errorf("expected delay[%d] (%v) > delay[%d] (%v)", i, delays[i], i-1, delays[i-1])
		}
	}
}

func TestScheduleRetry_Jitter(t *testing.T) {
	// Run multiple iterations and verify not all NextAttemptAt values are identical.
	// This is a probabilistic test; with jitter, repeated calls should vary.
	seen := make(map[int64]bool)
	for i := 0; i < 20; i++ {
		d := &store.Delivery{
			AttemptCount: 1,
			MaxRetries:   5,
		}
		scheduleRetry(d)
		seen[d.NextAttemptAt.UnixNano()] = true
	}
	// With 20 iterations and random jitter, we expect more than 1 unique value.
	if len(seen) < 2 {
		t.Error("expected jitter to produce varying NextAttemptAt values, but all were identical")
	}
}

func TestScheduleRetry_AttemptCountZero(t *testing.T) {
	// Edge case: AttemptCount = 0 with MaxRetries > 0 should not panic.
	// idx = 0 - 1 = -1, which is clamped to 0.
	d := &store.Delivery{
		AttemptCount: 0,
		MaxRetries:   5,
	}
	scheduleRetry(d) // should not panic

	if d.Status != store.StatusFailed {
		t.Errorf("expected status %d (StatusFailed), got %d", store.StatusFailed, d.Status)
	}
	if d.NextAttemptAt == nil {
		t.Fatal("expected NextAttemptAt to be set, got nil")
	}
}

func TestScheduleRetry_AttemptCountExceedsDelayTable(t *testing.T) {
	// When attemptCount-1 exceeds the retryDelays slice, idx should be clamped.
	d := &store.Delivery{
		AttemptCount: 100,
		MaxRetries:   200,
	}
	scheduleRetry(d) // should not panic

	if d.Status != store.StatusFailed {
		t.Errorf("expected status %d (StatusFailed), got %d", store.StatusFailed, d.Status)
	}
	if d.NextAttemptAt == nil {
		t.Fatal("expected NextAttemptAt to be set")
	}

	// Should use the last entry in retryDelays (43200 seconds = 12h).
	lastDelay := retryDelays[len(retryDelays)-1]
	expectedMin := time.Duration(lastDelay) * time.Second
	expectedMax := time.Duration(float64(lastDelay)*1.1+1) * time.Second
	actual := time.Until(*d.NextAttemptAt)
	if actual < expectedMin-time.Second || actual > expectedMax+time.Second {
		t.Errorf("expected delay around %v-%v, got %v", expectedMin, expectedMax, actual)
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "shorter than limit",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "longer than limit",
			input:  "hello world",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 5,
			want:   "",
		},
		{
			name:   "zero limit",
			input:  "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "single character limit",
			input:  "hello",
			maxLen: 1,
			want:   "h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
