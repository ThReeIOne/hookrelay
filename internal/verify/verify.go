package verify

import "fmt"

// Verifier interface
type Verifier interface {
	Verify(config map[string]any, headers map[string]string, body []byte) error
}

// GetVerifier returns a Verifier for the given type, or error for unknown types
func GetVerifier(verifyType string) (Verifier, error) {
	switch verifyType {
	case "hmac_sha256":
		return &HMACSha256Verifier{}, nil
	case "hmac_sha1":
		return &HMACSha1Verifier{}, nil
	case "basic_auth":
		return &BasicAuthVerifier{}, nil
	case "bearer_token":
		return &BearerTokenVerifier{}, nil
	case "none":
		return &NoopVerifier{}, nil
	default:
		return nil, fmt.Errorf("unknown verify type: %s", verifyType)
	}
}

// helper for safe type assertion from config maps
func configString(config map[string]any, key string) (string, error) {
	v, ok := config[key]
	if !ok {
		return "", fmt.Errorf("missing config key: %s", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("config key %s is not a string", key)
	}
	return s, nil
}

func configFloat(config map[string]any, key string) (float64, bool) {
	v, ok := config[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}
