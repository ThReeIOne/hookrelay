package transform

import "fmt"

// Transformer interface for transforming webhook payloads.
type Transformer interface {
	Transform(payload map[string]any) (map[string]any, error)
}

// GetTransformer creates a Transformer from config. Uses safe type assertions.
func GetTransformer(config map[string]any) (Transformer, error) {
	if config == nil {
		return &PassthroughTransformer{}, nil
	}
	t, ok := config["type"].(string)
	if !ok {
		return &PassthroughTransformer{}, nil
	}
	switch t {
	case "jmespath":
		expr, ok := config["expression"].(string)
		if !ok {
			return nil, fmt.Errorf("jmespath transformer requires 'expression' string")
		}
		return NewJMESPathTransformer(expr)
	case "remap":
		mapping, ok := config["mapping"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("remap transformer requires 'mapping' object")
		}
		return &RemapTransformer{Mapping: mapping}, nil
	case "template":
		body, ok := config["body"].(string)
		if !ok {
			return nil, fmt.Errorf("template transformer requires 'body' string")
		}
		return NewTemplateTransformer(body)
	default:
		return &PassthroughTransformer{}, nil
	}
}
