package verify

// NoopVerifier is a no-op verifier that always succeeds.
// Intended for development and testing purposes only.
type NoopVerifier struct{}

func (v *NoopVerifier) Verify(config map[string]any, headers map[string]string, body []byte) error {
	return nil
}
