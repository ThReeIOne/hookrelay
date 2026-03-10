package verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// HMACSha256Verifier verifies HMAC-SHA256 signatures.
// It supports two formats:
//   - Stripe format: header value like "t=1614556828,v1=<hex>" with timestamp tolerance
//   - Simple format: header value like "sha256=<hex>" or raw hex
type HMACSha256Verifier struct{}

func (v *HMACSha256Verifier) Verify(config map[string]any, headers map[string]string, body []byte) error {
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

	// Detect Stripe-style format: t=<timestamp>,v1=<signature>
	if strings.Contains(signature, "t=") && strings.Contains(signature, "v1=") {
		return v.verifyStripeFormat(secret, signature, body, config)
	}

	// Simple format: sha256=<hex> or raw hex
	return v.verifySimpleFormat(secret, signature, body)
}

// verifyStripeFormat handles Stripe-style signatures: "t=1614556828,v1=<hex>"
func (v *HMACSha256Verifier) verifyStripeFormat(secret, signature string, body []byte, config map[string]any) error {
	parts := strings.Split(signature, ",")

	var timestamp string
	var sig string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestamp = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			sig = strings.TrimPrefix(part, "v1=")
		}
	}

	if timestamp == "" || sig == "" {
		return fmt.Errorf("invalid Stripe signature format")
	}

	// Check timestamp tolerance if configured
	toleranceSec, hasTolerance := configFloat(config, "tolerance")
	if hasTolerance && toleranceSec > 0 {
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid timestamp in signature: %w", err)
		}
		diff := math.Abs(float64(time.Now().Unix() - ts))
		if diff > toleranceSec {
			return fmt.Errorf("signature timestamp outside tolerance: %.0fs > %.0fs", diff, toleranceSec)
		}
	}

	// Stripe signed payload: "<timestamp>.<body>"
	signedPayload := []byte(timestamp + "." + string(body))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(signedPayload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedMAC), []byte(sig)) {
		return fmt.Errorf("HMAC-SHA256 signature mismatch (Stripe format)")
	}

	return nil
}

// verifySimpleFormat handles "sha256=<hex>" or raw hex signatures.
func (v *HMACSha256Verifier) verifySimpleFormat(secret, signature string, body []byte) error {
	sig := signature
	if strings.HasPrefix(sig, "sha256=") {
		sig = strings.TrimPrefix(sig, "sha256=")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedMAC), []byte(sig)) {
		return fmt.Errorf("HMAC-SHA256 signature mismatch")
	}

	return nil
}
