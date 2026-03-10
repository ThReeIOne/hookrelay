package transform

import (
	"fmt"

	"github.com/jmespath/go-jmespath"
)

// JMESPathTransformer evaluates a JMESPath expression against the payload.
type JMESPathTransformer struct {
	compiled *jmespath.JMESPath
}

// NewJMESPathTransformer compiles the given JMESPath expression and returns
// a ready-to-use transformer, or an error if the expression is invalid.
func NewJMESPathTransformer(expression string) (*JMESPathTransformer, error) {
	compiled, err := jmespath.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("invalid jmespath expression: %w", err)
	}
	return &JMESPathTransformer{compiled: compiled}, nil
}

func (t *JMESPathTransformer) Transform(payload map[string]any) (map[string]any, error) {
	result, err := t.compiled.Search(payload)
	if err != nil {
		return nil, fmt.Errorf("jmespath search failed: %w", err)
	}

	if result == nil {
		return map[string]any{}, nil
	}

	// If the result is already a map, return it directly.
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}

	// Otherwise, wrap the result in a map under "result" key.
	return map[string]any{"result": result}, nil
}
