package transform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// TemplateTransformer uses a Go text/template to transform the payload.
// The template receives the full payload as the dot (.) context.
// The rendered output is parsed back as JSON into a map[string]any.
type TemplateTransformer struct {
	tmpl *template.Template
}

// NewTemplateTransformer compiles the given Go template string and returns
// a ready-to-use transformer, or an error if the template is invalid.
func NewTemplateTransformer(body string) (*TemplateTransformer, error) {
	tmpl, err := template.New("transform").Parse(body)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	return &TemplateTransformer{tmpl: tmpl}, nil
}

func (t *TemplateTransformer) Transform(payload map[string]any) (map[string]any, error) {
	var buf bytes.Buffer

	if err := t.tmpl.Execute(&buf, payload); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("template output is not valid JSON: %w", err)
	}

	return result, nil
}
