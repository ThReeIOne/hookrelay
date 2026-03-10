package transform

import (
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// GetTransformer
// ---------------------------------------------------------------------------

func TestGetTransformer_NilConfig(t *testing.T) {
	tr, err := GetTransformer(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.(*PassthroughTransformer); !ok {
		t.Fatalf("expected *PassthroughTransformer, got %T", tr)
	}
}

func TestGetTransformer_UnknownType(t *testing.T) {
	cfg := map[string]any{"type": "nonexistent"}
	tr, err := GetTransformer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.(*PassthroughTransformer); !ok {
		t.Fatalf("expected *PassthroughTransformer, got %T", tr)
	}
}

func TestGetTransformer_NoTypeKey(t *testing.T) {
	cfg := map[string]any{"expression": "foo"}
	tr, err := GetTransformer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.(*PassthroughTransformer); !ok {
		t.Fatalf("expected *PassthroughTransformer, got %T", tr)
	}
}

func TestGetTransformer_TypeNotString(t *testing.T) {
	cfg := map[string]any{"type": 42}
	tr, err := GetTransformer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.(*PassthroughTransformer); !ok {
		t.Fatalf("expected *PassthroughTransformer, got %T", tr)
	}
}

func TestGetTransformer_ValidTypes(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]any
		wantType string
	}{
		{
			name: "jmespath",
			config: map[string]any{
				"type":       "jmespath",
				"expression": "name",
			},
			wantType: "*transform.JMESPathTransformer",
		},
		{
			name: "remap",
			config: map[string]any{
				"type":    "remap",
				"mapping": map[string]any{"out": "in"},
			},
			wantType: "*transform.RemapTransformer",
		},
		{
			name: "template",
			config: map[string]any{
				"type": "template",
				"body": `{"greeting":"hello"}`,
			},
			wantType: "*transform.TemplateTransformer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := GetTransformer(tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := fmt.Sprintf("%T", tr)
			if got != tt.wantType {
				t.Fatalf("expected type %s, got %s", tt.wantType, got)
			}
		})
	}
}

func TestGetTransformer_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
	}{
		{
			name:   "jmespath missing expression",
			config: map[string]any{"type": "jmespath"},
		},
		{
			name:   "jmespath expression wrong type",
			config: map[string]any{"type": "jmespath", "expression": 123},
		},
		{
			name:   "remap missing mapping",
			config: map[string]any{"type": "remap"},
		},
		{
			name:   "remap mapping wrong type",
			config: map[string]any{"type": "remap", "mapping": "not_a_map"},
		},
		{
			name:   "template missing body",
			config: map[string]any{"type": "template"},
		},
		{
			name:   "template body wrong type",
			config: map[string]any{"type": "template", "body": 456},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetTransformer(tt.config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PassthroughTransformer
// ---------------------------------------------------------------------------

func TestPassthroughTransformer(t *testing.T) {
	tr := &PassthroughTransformer{}
	payload := map[string]any{"key": "value", "count": 42}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got key=%v", result["key"])
	}
	if result["count"] != 42 {
		t.Errorf("expected count=42, got count=%v", result["count"])
	}
}

func TestPassthroughTransformer_NilPayload(t *testing.T) {
	tr := &PassthroughTransformer{}
	result, err := tr.Transform(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for nil payload, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// JMESPathTransformer
// ---------------------------------------------------------------------------

func TestJMESPathTransformer_SimpleFieldExtraction(t *testing.T) {
	tr, err := NewJMESPathTransformer("name")
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	payload := map[string]any{"name": "test", "age": 30}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A non-map result is wrapped under "result" key.
	if result["result"] != "test" {
		t.Errorf("expected result[\"result\"]=\"test\", got %v", result["result"])
	}
}

func TestJMESPathTransformer_NestedPath(t *testing.T) {
	tr, err := NewJMESPathTransformer("data.user")
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	payload := map[string]any{
		"data": map[string]any{
			"user": map[string]any{
				"id": 1,
			},
		},
	}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// data.user is already a map, so it should be returned directly.
	id, ok := result["id"]
	if !ok {
		t.Fatal("expected 'id' key in result")
	}
	// JMESPath may return float64 for numbers in some cases.
	switch v := id.(type) {
	case int:
		if v != 1 {
			t.Errorf("expected id=1, got %d", v)
		}
	case float64:
		if v != 1.0 {
			t.Errorf("expected id=1.0, got %f", v)
		}
	default:
		t.Errorf("unexpected type for id: %T = %v", id, id)
	}
}

func TestJMESPathTransformer_NilResult(t *testing.T) {
	tr, err := NewJMESPathTransformer("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	payload := map[string]any{"name": "test"}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map for nil result, got %v", result)
	}
}

func TestJMESPathTransformer_InvalidExpression(t *testing.T) {
	_, err := NewJMESPathTransformer("|||invalid|||")
	if err == nil {
		t.Fatal("expected error for invalid expression, got nil")
	}
}

func TestJMESPathTransformer_ArrayResult(t *testing.T) {
	tr, err := NewJMESPathTransformer("items[*].name")
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	payload := map[string]any{
		"items": []any{
			map[string]any{"name": "a"},
			map[string]any{"name": "b"},
		},
	}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Array result should be wrapped in "result" key.
	arr, ok := result["result"].([]any)
	if !ok {
		t.Fatalf("expected []any under 'result', got %T", result["result"])
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 items, got %d", len(arr))
	}
}

// ---------------------------------------------------------------------------
// RemapTransformer
// ---------------------------------------------------------------------------

func TestRemapTransformer_NestedPaths(t *testing.T) {
	tr := &RemapTransformer{
		Mapping: map[string]any{
			"order_id": "data.order.id",
			"amount":   "data.amount",
		},
	}
	payload := map[string]any{
		"data": map[string]any{
			"order": map[string]any{
				"id": "ORD-123",
			},
			"amount": 99.95,
		},
	}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["order_id"] != "ORD-123" {
		t.Errorf("expected order_id=ORD-123, got %v", result["order_id"])
	}
	if result["amount"] != 99.95 {
		t.Errorf("expected amount=99.95, got %v", result["amount"])
	}
}

func TestRemapTransformer_MissingSourcePath(t *testing.T) {
	tr := &RemapTransformer{
		Mapping: map[string]any{
			"exists":  "a",
			"missing": "b.c.d",
		},
	}
	payload := map[string]any{"a": "hello"}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["exists"] != "hello" {
		t.Errorf("expected exists=hello, got %v", result["exists"])
	}
	if _, found := result["missing"]; found {
		t.Errorf("expected 'missing' key to be absent, but it was present with value %v", result["missing"])
	}
}

func TestRemapTransformer_NonStringMappingValue(t *testing.T) {
	tr := &RemapTransformer{
		Mapping: map[string]any{
			"bad": 12345,
		},
	}
	payload := map[string]any{"a": "hello"}
	_, err := tr.Transform(payload)
	if err == nil {
		t.Fatal("expected error for non-string mapping value, got nil")
	}
}

func TestRemapTransformer_EmptyMapping(t *testing.T) {
	tr := &RemapTransformer{
		Mapping: map[string]any{},
	}
	payload := map[string]any{"a": "hello"}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestRemapTransformer_TopLevelField(t *testing.T) {
	tr := &RemapTransformer{
		Mapping: map[string]any{
			"out": "name",
		},
	}
	payload := map[string]any{"name": "Alice"}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["out"] != "Alice" {
		t.Errorf("expected out=Alice, got %v", result["out"])
	}
}

// ---------------------------------------------------------------------------
// TemplateTransformer
// ---------------------------------------------------------------------------

func TestTemplateTransformer_RendersPayloadData(t *testing.T) {
	tmplBody := `{"greeting":"Hello, {{.name}}!", "count":{{.count}}}`
	tr, err := NewTemplateTransformer(tmplBody)
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	payload := map[string]any{"name": "World", "count": 5}
	result, err := tr.Transform(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["greeting"] != "Hello, World!" {
		t.Errorf("expected greeting='Hello, World!', got %v", result["greeting"])
	}
	// JSON numbers decode as float64.
	if result["count"] != float64(5) {
		t.Errorf("expected count=5, got %v (type %T)", result["count"], result["count"])
	}
}

func TestTemplateTransformer_InvalidTemplate(t *testing.T) {
	_, err := NewTemplateTransformer(`{{.unclosed`)
	if err == nil {
		t.Fatal("expected error for invalid template, got nil")
	}
}

func TestTemplateTransformer_NonJSONOutput(t *testing.T) {
	// Template that renders valid template syntax but invalid JSON.
	tr, err := NewTemplateTransformer(`not json at all`)
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	_, err = tr.Transform(map[string]any{})
	if err == nil {
		t.Fatal("expected error for non-JSON template output, got nil")
	}
}

func TestTemplateTransformer_EmptyObject(t *testing.T) {
	tr, err := NewTemplateTransformer(`{}`)
	if err != nil {
		t.Fatalf("unexpected error creating transformer: %v", err)
	}
	result, err := tr.Transform(map[string]any{"ignored": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}
