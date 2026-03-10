package router

import (
	"context"
	"log/slog"

	"github.com/hookrelay/hookrelay/internal/store"
)

// Router matches events to subscriptions and creates delivery records.
type Router struct {
	store store.Store
}

// NewRouter creates a new Router.
func NewRouter(s store.Store) *Router {
	return &Router{store: s}
}

// Route matches the event against all active subscriptions for its source
// and creates a delivery record for each match.
func (r *Router) Route(ctx context.Context, event *store.Event) (int, error) {
	subs, err := r.store.ListActiveSubscriptions(ctx, event.SourceID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, sub := range subs {
		if !matchFilter(sub.EventFilter, event.EventType) {
			continue
		}
		_, err := r.store.CreateDelivery(ctx, &store.Delivery{
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			Status:         store.StatusPending,
			MaxRetries:     sub.MaxRetries,
			NextAttemptAt:  nil, // nil = deliver immediately
		})
		if err != nil {
			slog.Error("create delivery failed",
				"event_id", event.ID,
				"sub_id", sub.ID,
				"error", err,
			)
			continue
		}
		count++
	}
	return count, nil
}

func matchFilter(filters []string, eventType string) bool {
	for _, pattern := range filters {
		if matchGlob(pattern, eventType) {
			return true
		}
	}
	return false
}
