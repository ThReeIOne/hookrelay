package transform

import (
	"fmt"
	"strings"
)

// RemapTransformer maps fields from the input payload to new output fields.
// Mapping keys are output field names. Mapping values are dot-separated paths
// into the input payload (e.g. "data.user.name").
type RemapTransformer struct {
	Mapping map[string]any
}

func (t *RemapTransformer) Transform(payload map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	for outputKey, pathVal := range t.Mapping {
		path, ok := pathVal.(string)
		if !ok {
			return nil, fmt.Errorf("remap: mapping value for key %q must be a string, got %T", outputKey, pathVal)
		}

		value, found := getNestedValue(payload, path)
		if found {
			result[outputKey] = value
		}
	}

	return result, nil
}

// getNestedValue walks a dot-separated path through nested maps.
// Returns the value and true if found, or nil and false otherwise.
func getNestedValue(data map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}
