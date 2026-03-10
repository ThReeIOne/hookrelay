package store

import (
	"context"
	"time"
)

// Store defines the persistence interface for HookRelay.
type Store interface {
	// Source CRUD
	CreateSource(ctx context.Context, s *Source) (*Source, error)
	GetSource(ctx context.Context, id int64) (*Source, error)
	GetSourceByName(ctx context.Context, name string) (*Source, error)
	ListSources(ctx context.Context) ([]*Source, error)
	UpdateSource(ctx context.Context, s *Source) error
	DeleteSource(ctx context.Context, id int64) error

	// Event operations
	CreateEvent(ctx context.Context, e *Event) (*Event, error)
	GetEvent(ctx context.Context, id int64) (*Event, error)
	FindEventByIdempotencyKey(ctx context.Context, sourceID int64, key string) (*Event, error)
	ListEvents(ctx context.Context, filter EventFilter) ([]*Event, int64, error) // returns events + total count

	// Subscription CRUD
	CreateSubscription(ctx context.Context, s *Subscription) (*Subscription, error)
	GetSubscription(ctx context.Context, id int64) (*Subscription, error)
	ListSubscriptions(ctx context.Context, sourceID int64) ([]*Subscription, error)
	ListActiveSubscriptions(ctx context.Context, sourceID int64) ([]*Subscription, error)
	UpdateSubscription(ctx context.Context, s *Subscription) error
	DeleteSubscription(ctx context.Context, id int64) error
	GetSubscriptionStats(ctx context.Context, id int64) (*SubscriptionStats, error)

	// Delivery operations
	CreateDelivery(ctx context.Context, d *Delivery) (*Delivery, error)
	GetDelivery(ctx context.Context, id int64) (*Delivery, error)
	UpdateDelivery(ctx context.Context, d *Delivery) error
	FetchPendingDeliveries(ctx context.Context, limit int) ([]*Delivery, error) // FOR UPDATE SKIP LOCKED
	ListDeliveriesByEvent(ctx context.Context, eventID int64) ([]*Delivery, error)
	ListDeadLetters(ctx context.Context, filter DeliveryFilter) ([]*Delivery, int64, error)
	RetryDeadLetter(ctx context.Context, deliveryID int64) error
	BatchRetryDeadLetters(ctx context.Context, deliveryIDs []int64) error

	// DeliveryAttempt operations
	CreateDeliveryAttempt(ctx context.Context, a *DeliveryAttempt) (*DeliveryAttempt, error)
	ListDeliveryAttempts(ctx context.Context, deliveryID int64) ([]*DeliveryAttempt, error)

	// Stats & analytics
	GetOverviewStats(ctx context.Context) (*OverviewStats, error)
	GetThroughput(ctx context.Context, start, end time.Time, granularity string) ([]*ThroughputPoint, error)

	// Health check
	Ping(ctx context.Context) error
}
