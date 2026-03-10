package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRateLimiter_ZeroAndNegativeRPS(t *testing.T) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()

	tests := []struct {
		name string
		rps  int
	}{
		{name: "zero rps rejects", rps: 0},
		{name: "negative rps rejects", rps: -1},
		{name: "very negative rps rejects", rps: -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rl.Allow(ctx, 1, tt.rps)
			if got != false {
				t.Errorf("Allow(ctx, 1, %d) = %v, want false", tt.rps, got)
			}
		})
	}
}

func TestMemoryRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()
	var subID int64 = 42
	rps := 5

	// Should allow exactly rps requests.
	for i := 0; i < rps; i++ {
		if !rl.Allow(ctx, subID, rps) {
			t.Fatalf("request %d should be allowed (limit %d)", i+1, rps)
		}
	}

	// The next request should be rejected.
	if rl.Allow(ctx, subID, rps) {
		t.Errorf("request %d should be rejected (limit %d)", rps+1, rps)
	}
}

func TestMemoryRateLimiter_RejectsBeyondLimit(t *testing.T) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()
	var subID int64 = 10
	rps := 3

	for i := 0; i < rps; i++ {
		rl.Allow(ctx, subID, rps)
	}

	// Several additional requests should all be rejected.
	for i := 0; i < 5; i++ {
		if rl.Allow(ctx, subID, rps) {
			t.Errorf("over-limit request %d should be rejected", i+1)
		}
	}
}

func TestMemoryRateLimiter_IndependentSubscriptions(t *testing.T) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()
	var subA int64 = 1
	var subB int64 = 2
	rps := 2

	// Exhaust the limit for subscription A.
	for i := 0; i < rps; i++ {
		if !rl.Allow(ctx, subA, rps) {
			t.Fatalf("subA request %d should be allowed", i+1)
		}
	}

	// Subscription A should be rejected.
	if rl.Allow(ctx, subA, rps) {
		t.Error("subA should be rejected after exhausting limit")
	}

	// Subscription B should still be allowed (independent window).
	for i := 0; i < rps; i++ {
		if !rl.Allow(ctx, subB, rps) {
			t.Errorf("subB request %d should be allowed (independent of subA)", i+1)
		}
	}
}

func TestMemoryRateLimiter_WindowResets(t *testing.T) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()
	var subID int64 = 99
	rps := 2

	// Exhaust the limit.
	for i := 0; i < rps; i++ {
		if !rl.Allow(ctx, subID, rps) {
			t.Fatalf("initial request %d should be allowed", i+1)
		}
	}

	// Confirm we're at the limit.
	if rl.Allow(ctx, subID, rps) {
		t.Fatal("should be rejected at the limit")
	}

	// Wait for the window to expire (just over 1 second).
	time.Sleep(1100 * time.Millisecond)

	// After the window resets, requests should be allowed again.
	if !rl.Allow(ctx, subID, rps) {
		t.Error("request should be allowed after window reset")
	}
}

func TestMemoryRateLimiter_SingleRPS(t *testing.T) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()
	var subID int64 = 77

	// With rps=1, only one request per second is allowed.
	if !rl.Allow(ctx, subID, 1) {
		t.Error("first request with rps=1 should be allowed")
	}
	if rl.Allow(ctx, subID, 1) {
		t.Error("second request with rps=1 should be rejected")
	}
}
