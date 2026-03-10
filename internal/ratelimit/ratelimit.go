package ratelimit

import "context"

// RateLimiter controls the rate of webhook deliveries per subscription.
type RateLimiter interface {
	Allow(ctx context.Context, subscriptionID int64, rps int) bool
}
