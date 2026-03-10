package ingress

import (
	"net/http"
	"testing"

	"github.com/hookrelay/hookrelay/internal/store"
)

// ---------------------------------------------------------------------------
// flattenHeaders
// ---------------------------------------------------------------------------

func TestFlattenHeaders_MultiValueToSingleValue(t *testing.T) {
	h := http.Header{
		"Content-Type":   {"application/json"},
		"X-Custom":       {"value1", "value2"},
		"X-Mixed-Case":   {"mixed"},
		"UPPER":          {"upper"},
		"Accept-Charset": {"utf-8"},
	}
	got := flattenHeaders(h)

	tests := []struct {
		key  string
		want string
	}{
		{"content-type", "application/json"},
		{"x-custom", "value1"},      // only first value kept
		{"x-mixed-case", "mixed"},   // key lowercased
		{"upper", "upper"},          // key lowercased
		{"accept-charset", "utf-8"}, // key lowercased
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if v, ok := got[tc.key]; !ok {
				t.Errorf("missing key %q", tc.key)
			} else if v != tc.want {
				t.Errorf("flattenHeaders[%q] = %q, want %q", tc.key, v, tc.want)
			}
		})
	}
}

func TestFlattenHeaders_EmptyHeaders(t *testing.T) {
	got := flattenHeaders(http.Header{})
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestFlattenHeaders_NilHeader(t *testing.T) {
	got := flattenHeaders(nil)
	if got == nil {
		t.Fatal("expected non-nil map for nil input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// jsonPathGet
// ---------------------------------------------------------------------------

func TestJsonPathGet(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		path string
		want string
	}{
		{
			name: "simple top-level string",
			data: map[string]any{"type": "order.created"},
			path: "type",
			want: "order.created",
		},
		{
			name: "nested path",
			data: map[string]any{
				"data": map[string]any{
					"order": map[string]any{
						"id": "123",
					},
				},
			},
			path: "data.order.id",
			want: "123",
		},
		{
			name: "deeply nested path",
			data: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": map[string]any{
							"d": "deep",
						},
					},
				},
			},
			path: "a.b.c.d",
			want: "deep",
		},
		{
			name: "missing top-level key",
			data: map[string]any{"type": "order.created"},
			path: "missing",
			want: "",
		},
		{
			name: "missing nested key",
			data: map[string]any{
				"data": map[string]any{
					"order": map[string]any{},
				},
			},
			path: "data.order.id",
			want: "",
		},
		{
			name: "path through non-map value",
			data: map[string]any{
				"data": "scalar",
			},
			path: "data.nested.key",
			want: "",
		},
		{
			name: "float64 integer value without trailing zero",
			data: map[string]any{"count": float64(7)},
			path: "count",
			want: "7",
		},
		{
			name: "float64 decimal value",
			data: map[string]any{"price": float64(19.99)},
			path: "price",
			want: "19.99",
		},
		{
			name: "object value serialized as JSON",
			data: map[string]any{
				"nested": map[string]any{
					"key": "value",
				},
			},
			path: "nested",
			want: `{"key":"value"}`,
		},
		{
			name: "boolean value serialized as JSON",
			data: map[string]any{"active": true},
			path: "active",
			want: "true",
		},
		{
			name: "nil value returns empty string",
			data: map[string]any{"key": nil},
			path: "key",
			want: "",
		},
		{
			name: "empty path splits to single empty part",
			data: map[string]any{"": "root"},
			path: "",
			want: "root",
		},
		{
			name: "empty map",
			data: map[string]any{},
			path: "any",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonPathGet(tc.data, tc.path)
			if got != tc.want {
				t.Errorf("jsonPathGet(%v, %q) = %q, want %q", tc.data, tc.path, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractEventType
// ---------------------------------------------------------------------------

func TestExtractEventType(t *testing.T) {
	tests := []struct {
		name    string
		source  *store.Source
		headers map[string]string
		payload map[string]any
		want    string
	}{
		{
			name: "prefers header when EventTypeHeader is set and header exists",
			source: &store.Source{
				EventTypeHeader: "X-Event-Type",
				EventTypePath:   "event.type",
			},
			headers: map[string]string{
				"x-event-type": "push",
			},
			payload: map[string]any{
				"event": map[string]any{"type": "pull_request"},
			},
			want: "push",
		},
		{
			name: "falls back to payload path when header not found",
			source: &store.Source{
				EventTypeHeader: "X-Event-Type",
				EventTypePath:   "event.type",
			},
			headers: map[string]string{},
			payload: map[string]any{
				"event": map[string]any{"type": "pull_request"},
			},
			want: "pull_request",
		},
		{
			name: "falls back to payload path when EventTypeHeader is empty string",
			source: &store.Source{
				EventTypeHeader: "",
				EventTypePath:   "type",
			},
			headers: map[string]string{
				"x-event-type": "from-header",
			},
			payload: map[string]any{"type": "from-payload"},
			want:    "from-payload",
		},
		{
			name: "falls back to payload when header value is empty",
			source: &store.Source{
				EventTypeHeader: "X-Event-Type",
				EventTypePath:   "type",
			},
			headers: map[string]string{
				"x-event-type": "",
			},
			payload: map[string]any{"type": "payload-type"},
			want:    "payload-type",
		},
		{
			name: "returns unknown when neither header nor payload path works",
			source: &store.Source{
				EventTypeHeader: "X-Event-Type",
				EventTypePath:   "event.type",
			},
			headers: map[string]string{},
			payload: map[string]any{},
			want:    "unknown",
		},
		{
			name: "returns unknown when both configs are empty",
			source: &store.Source{
				EventTypeHeader: "",
				EventTypePath:   "",
			},
			headers: map[string]string{"x-event-type": "ignored"},
			payload: map[string]any{"type": "also-ignored"},
			want:    "unknown",
		},
		{
			name: "header lookup is case-insensitive (lowercased)",
			source: &store.Source{
				EventTypeHeader: "X-GitHub-Event",
			},
			headers: map[string]string{
				"x-github-event": "issues",
			},
			payload: map[string]any{},
			want:    "issues",
		},
		{
			name: "only payload path set, no header config",
			source: &store.Source{
				EventTypePath: "action",
			},
			headers: map[string]string{},
			payload: map[string]any{"action": "opened"},
			want:    "opened",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractEventType(tc.source, tc.headers, tc.payload)
			if got != tc.want {
				t.Errorf("extractEventType() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractIdempotencyKey
// ---------------------------------------------------------------------------

func TestExtractIdempotencyKey(t *testing.T) {
	tests := []struct {
		name    string
		source  *store.Source
		headers map[string]string
		payload map[string]any
		want    string
	}{
		{
			name: "prefers header when IdempotencyHeader is set and header exists",
			source: &store.Source{
				IdempotencyHeader: "X-Idempotency-Key",
				IdempotencyPath:   "meta.id",
			},
			headers: map[string]string{
				"x-idempotency-key": "header-key-123",
			},
			payload: map[string]any{
				"meta": map[string]any{"id": "payload-key-456"},
			},
			want: "header-key-123",
		},
		{
			name: "falls back to payload path when header not found",
			source: &store.Source{
				IdempotencyHeader: "X-Idempotency-Key",
				IdempotencyPath:   "meta.request_id",
			},
			headers: map[string]string{},
			payload: map[string]any{
				"meta": map[string]any{"request_id": "req-789"},
			},
			want: "req-789",
		},
		{
			name: "falls back to payload when header value is empty",
			source: &store.Source{
				IdempotencyHeader: "X-Idempotency-Key",
				IdempotencyPath:   "id",
			},
			headers: map[string]string{
				"x-idempotency-key": "",
			},
			payload: map[string]any{"id": "fallback-id"},
			want:    "fallback-id",
		},
		{
			name: "returns empty string when neither works",
			source: &store.Source{
				IdempotencyHeader: "X-Idempotency-Key",
				IdempotencyPath:   "meta.id",
			},
			headers: map[string]string{},
			payload: map[string]any{},
			want:    "",
		},
		{
			name: "returns empty string when both configs are empty",
			source: &store.Source{
				IdempotencyHeader: "",
				IdempotencyPath:   "",
			},
			headers: map[string]string{"x-idempotency-key": "ignored"},
			payload: map[string]any{"id": "also-ignored"},
			want:    "",
		},
		{
			name: "header lookup is case-insensitive (lowercased)",
			source: &store.Source{
				IdempotencyHeader: "X-Request-ID",
			},
			headers: map[string]string{
				"x-request-id": "abc-def",
			},
			payload: map[string]any{},
			want:    "abc-def",
		},
		{
			name: "only payload path set, extracts from payload",
			source: &store.Source{
				IdempotencyPath: "data.unique_id",
			},
			headers: map[string]string{},
			payload: map[string]any{
				"data": map[string]any{"unique_id": "uid-001"},
			},
			want: "uid-001",
		},
		{
			name: "payload path returns empty for missing key yields empty string",
			source: &store.Source{
				IdempotencyPath: "nonexistent.path",
			},
			headers: map[string]string{},
			payload: map[string]any{},
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIdempotencyKey(tc.source, tc.headers, tc.payload)
			if got != tc.want {
				t.Errorf("extractIdempotencyKey() = %q, want %q", got, tc.want)
			}
		})
	}
}
