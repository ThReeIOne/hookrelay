package router

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		// "*" matches everything
		{name: "star matches simple event", pattern: "*", value: "payment.succeeded", want: true},
		{name: "star matches dotted event", pattern: "*", value: "payment.intent.succeeded", want: true},
		{name: "star matches single word", pattern: "*", value: "created", want: true},
		{name: "star matches empty value", pattern: "*", value: "", want: true},

		// "**" matches everything
		{name: "doublestar matches simple event", pattern: "**", value: "payment.succeeded", want: true},
		{name: "doublestar matches dotted event", pattern: "**", value: "payment.intent.succeeded", want: true},
		{name: "doublestar matches single word", pattern: "**", value: "created", want: true},
		{name: "doublestar matches empty value", pattern: "**", value: "", want: true},

		// "payment.*" — single-level wildcard
		{name: "payment.star matches payment.succeeded", pattern: "payment.*", value: "payment.succeeded", want: true},
		{name: "payment.star matches payment.failed", pattern: "payment.*", value: "payment.failed", want: true},
		{name: "payment.star does not match multi-level", pattern: "payment.*", value: "payment.intent.succeeded", want: false},
		{name: "payment.star does not match other prefix", pattern: "payment.*", value: "order.created", want: false},

		// "payment.**" — multi-level wildcard
		{name: "payment.doublestar matches payment.succeeded", pattern: "payment.**", value: "payment.succeeded", want: true},
		{name: "payment.doublestar matches multi-level", pattern: "payment.**", value: "payment.intent.succeeded", want: true},
		{name: "payment.doublestar does not match other prefix", pattern: "payment.**", value: "order.created", want: false},

		// exact match
		{name: "exact match succeeds", pattern: "payment.succeeded", value: "payment.succeeded", want: true},
		{name: "exact match fails on different event", pattern: "payment.succeeded", value: "payment.failed", want: false},
		{name: "exact match fails on prefix match", pattern: "payment.succeeded", value: "payment.succeeded.extra", want: false},

		// edge cases: empty pattern
		{name: "empty pattern matches empty value", pattern: "", value: "", want: true},
		{name: "empty pattern does not match non-empty value", pattern: "", value: "payment.succeeded", want: false},

		// edge cases: empty value
		{name: "non-empty pattern does not match empty value", pattern: "payment.*", value: "", want: false},

		// edge cases: pattern with no dots
		{name: "single word pattern matches same word", pattern: "created", value: "created", want: true},
		{name: "single word pattern does not match different word", pattern: "created", value: "deleted", want: false},
		{name: "single word pattern does not match dotted value", pattern: "created", value: "order.created", want: false},

		// edge cases: value with no dots
		{name: "dotted pattern does not match undotted value", pattern: "payment.*", value: "payment", want: false},

		// middle wildcard
		{name: "middle star wildcard", pattern: "payment.*.created", value: "payment.intent.created", want: true},
		{name: "middle star wildcard mismatch", pattern: "payment.*.created", value: "payment.intent.failed", want: false},

		// middle double-star consumes rest
		{name: "middle doublestar matches remaining", pattern: "payment.**.created", value: "payment.intent.sub.created", want: true},
		{name: "middle doublestar also matches shorter", pattern: "payment.**.created", value: "payment.created", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestMatchFilter(t *testing.T) {
	tests := []struct {
		name      string
		filters   []string
		eventType string
		want      bool
	}{
		// Multiple patterns: matches payment events
		{
			name:      "matches first pattern in list",
			filters:   []string{"payment.*", "refund.*"},
			eventType: "payment.succeeded",
			want:      true,
		},
		// Multiple patterns: matches refund events
		{
			name:      "matches second pattern in list",
			filters:   []string{"payment.*", "refund.*"},
			eventType: "refund.created",
			want:      true,
		},
		// No match returns false
		{
			name:      "no match returns false",
			filters:   []string{"payment.*", "refund.*"},
			eventType: "order.created",
			want:      false,
		},
		// Empty filter slice returns false
		{
			name:      "empty filter returns false",
			filters:   []string{},
			eventType: "payment.succeeded",
			want:      false,
		},
		// Nil filter slice returns false
		{
			name:      "nil filter returns false",
			filters:   nil,
			eventType: "payment.succeeded",
			want:      false,
		},
		// Single exact match filter
		{
			name:      "single exact match filter",
			filters:   []string{"payment.succeeded"},
			eventType: "payment.succeeded",
			want:      true,
		},
		// Wildcard filter matches all
		{
			name:      "wildcard filter matches all",
			filters:   []string{"*"},
			eventType: "anything.here",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchFilter(tt.filters, tt.eventType)
			if got != tt.want {
				t.Errorf("matchFilter(%v, %q) = %v, want %v", tt.filters, tt.eventType, got, tt.want)
			}
		})
	}
}
