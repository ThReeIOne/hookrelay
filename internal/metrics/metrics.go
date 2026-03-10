package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventsReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hookrelay_events_received_total",
			Help: "Total number of events received",
		},
		[]string{"source"},
	)

	EventsRejectedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hookrelay_events_rejected_total",
			Help: "Total number of events rejected (verify failed / dedup)",
		},
		[]string{"reason"},
	)

	DeliveriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hookrelay_deliveries_total",
			Help: "Total number of delivery attempts",
		},
		[]string{"status"},
	)

	DeliveryDurationMs = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "hookrelay_delivery_duration_ms",
			Help:    "Delivery duration in milliseconds",
			Buckets: []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		},
	)

	DeliveryQueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hookrelay_delivery_queue_depth",
			Help: "Number of pending deliveries in queue",
		},
	)

	DeadLettersTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hookrelay_dead_letters_total",
			Help: "Total number of deliveries sent to dead letter queue",
		},
	)

	RetryTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hookrelay_retry_total",
			Help: "Total number of delivery retries",
		},
	)

	RateLimitedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "hookrelay_rate_limited_total",
			Help: "Total number of deliveries rate limited",
		},
	)
)
