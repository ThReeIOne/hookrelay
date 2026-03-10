package verify

import (
	"fmt"
	"strings"
)

// BearerTokenVerifier verifies Bearer Token authentication.
// Config requires "token" and optionally "header" (defaults to "authorization").
// Checks the header for "Bearer <token>".
type BearerTokenVerifier struct{}

func (v *BearerTokenVerifier) Verify(config map[string]any, headers map[string]string, body []byte) error {
	expectedToken, err := configString(config, "token")
	if err != nil {
		return err
	}

	// Default header is "authorization", but can be overridden
	headerName := "authorization"
	if h, err := configString(config, "header"); err == nil && h != "" {
		headerName = strings.ToLower(h)
	}

	headerValue, ok := headers[headerName]
	if !ok {
		return fmt.Errorf("missing header: %s", headerName)
	}

	expectedValue := "Bearer " + expectedToken
	if headerValue != expectedValue {
		return fmt.Errorf("bearer token mismatch")
	}

	return nil
}
