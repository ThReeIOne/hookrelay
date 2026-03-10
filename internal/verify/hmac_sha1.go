package verify

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

// HMACSha1Verifier verifies HMAC-SHA1 signatures in the GitHub format.
// The signature header value is expected to start with "sha1=" followed by
// the hex-encoded HMAC-SHA1 digest.
type HMACSha1Verifier struct{}

func (v *HMACSha1Verifier) Verify(config map[string]any, headers map[string]string, body []byte) error {
	secret, err := configString(config, "secret")
	if err != nil {
		return err
	}

	headerName, err := configString(config, "header")
	if err != nil {
		return err
	}

	signature, ok := headers[strings.ToLower(headerName)]
	if !ok {
		return fmt.Errorf("missing signature header: %s", headerName)
	}

	if !strings.HasPrefix(signature, "sha1=") {
		return fmt.Errorf("HMAC-SHA1 signature must start with 'sha1='")
	}

	sig := strings.TrimPrefix(signature, "sha1=")

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedMAC), []byte(sig)) {
		return fmt.Errorf("HMAC-SHA1 signature mismatch")
	}

	return nil
}
