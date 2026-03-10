package transform

// PassthroughTransformer returns the payload unchanged.
type PassthroughTransformer struct{}

func (t *PassthroughTransformer) Transform(payload map[string]any) (map[string]any, error) {
	return payload, nil
}
