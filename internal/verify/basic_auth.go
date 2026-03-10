package verify

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// BasicAuthVerifier verifies HTTP Basic Authentication.
// Config requires "username" and "password" keys.
// Checks the Authorization header for "Basic <base64(username:password)>".
type BasicAuthVerifier struct{}

func (v *BasicAuthVerifier) Verify(config map[string]any, headers map[string]string, body []byte) error {
	expectedUsername, err := configString(config, "username")
	if err != nil {
		return err
	}

	expectedPassword, err := configString(config, "password")
	if err != nil {
		return err
	}

	authHeader, ok := headers["authorization"]
	if !ok {
		return fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return fmt.Errorf("Authorization header must start with 'Basic '")
	}

	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("invalid base64 in Authorization header: %w", err)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid Basic Auth format: expected username:password")
	}

	if parts[0] != expectedUsername || parts[1] != expectedPassword {
		return fmt.Errorf("Basic Auth credentials mismatch")
	}

	return nil
}
