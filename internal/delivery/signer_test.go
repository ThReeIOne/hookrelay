package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSign_Format(t *testing.T) {
	result := sign("secret", "1234567890", []byte(`{"event":"test"}`))

	// Must contain "t=" and ",v1="
	if !strings.HasPrefix(result, "t=1234567890,v1=") {
		t.Errorf("unexpected format: %s", result)
	}

	parts := strings.SplitN(result, ",", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts separated by comma, got %d in %q", len(parts), result)
	}

	tPart := parts[0]
	if tPart != "t=1234567890" {
		t.Errorf("expected t=1234567890, got %s", tPart)
	}

	v1Part := parts[1]
	if !strings.HasPrefix(v1Part, "v1=") {
		t.Errorf("expected v1= prefix, got %s", v1Part)
	}

	// The hex signature should be 64 characters (SHA-256 = 32 bytes = 64 hex chars).
	hexSig := strings.TrimPrefix(v1Part, "v1=")
	if len(hexSig) != 64 {
		t.Errorf("expected 64 hex chars, got %d: %s", len(hexSig), hexSig)
	}

	// Verify it's valid hex.
	_, err := hex.DecodeString(hexSig)
	if err != nil {
		t.Errorf("v1 value is not valid hex: %v", err)
	}
}

func TestSign_CorrectHMAC(t *testing.T) {
	secret := "my-webhook-secret"
	timestamp := "1700000000"
	body := []byte(`{"action":"created","id":42}`)

	result := sign(secret, timestamp, body)

	// Manually compute expected HMAC-SHA256.
	payload := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	expected := "t=" + timestamp + ",v1=" + expectedSig

	if result != expected {
		t.Errorf("sign() = %s, want %s", result, expected)
	}
}

func TestSign_DifferentSecrets(t *testing.T) {
	timestamp := "1700000000"
	body := []byte(`{"event":"test"}`)

	sig1 := sign("secret-one", timestamp, body)
	sig2 := sign("secret-two", timestamp, body)

	if sig1 == sig2 {
		t.Error("expected different signatures for different secrets, but they are equal")
	}
}

func TestSign_DifferentBodies(t *testing.T) {
	secret := "shared-secret"
	timestamp := "1700000000"

	sig1 := sign(secret, timestamp, []byte(`{"event":"created"}`))
	sig2 := sign(secret, timestamp, []byte(`{"event":"deleted"}`))

	if sig1 == sig2 {
		t.Error("expected different signatures for different bodies, but they are equal")
	}
}

func TestSign_DifferentTimestamps(t *testing.T) {
	secret := "shared-secret"
	body := []byte(`{"event":"test"}`)

	sig1 := sign(secret, "1000000000", body)
	sig2 := sign(secret, "2000000000", body)

	if sig1 == sig2 {
		t.Error("expected different signatures for different timestamps, but they are equal")
	}
}

func TestSign_EmptyBody(t *testing.T) {
	result := sign("secret", "12345", []byte{})

	// Should still produce a valid signature.
	if !strings.HasPrefix(result, "t=12345,v1=") {
		t.Errorf("unexpected format for empty body: %s", result)
	}

	// Verify against manual computation.
	payload := "12345."
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	expected := "t=12345,v1=" + expectedSig

	if result != expected {
		t.Errorf("sign() = %s, want %s", result, expected)
	}
}

func TestSign_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		timestamp string
		body      []byte
	}{
		{"basic", "sec", "100", []byte("body")},
		{"unicode body", "key", "200", []byte(`{"name":"cafe\u0301"}`)},
		{"long secret", strings.Repeat("x", 256), "300", []byte("data")},
		{"empty secret", "", "400", []byte("data")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sign(tt.secret, tt.timestamp, tt.body)

			// Verify format.
			prefix := "t=" + tt.timestamp + ",v1="
			if !strings.HasPrefix(result, prefix) {
				t.Fatalf("expected prefix %q, got %q", prefix, result)
			}

			// Verify correctness.
			payload := tt.timestamp + "." + string(tt.body)
			mac := hmac.New(sha256.New, []byte(tt.secret))
			mac.Write([]byte(payload))
			expectedSig := hex.EncodeToString(mac.Sum(nil))
			expected := prefix + expectedSig

			if result != expected {
				t.Errorf("sign(%q, %q, %q) = %s, want %s", tt.secret, tt.timestamp, tt.body, result, expected)
			}
		})
	}
}
