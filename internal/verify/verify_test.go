package verify

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// GetVerifier
// ---------------------------------------------------------------------------

func TestGetVerifier(t *testing.T) {
	tests := []struct {
		name       string
		verifyType string
		wantType   string
		wantErr    bool
	}{
		{
			name:       "hmac_sha256 returns HMACSha256Verifier",
			verifyType: "hmac_sha256",
			wantType:   "*verify.HMACSha256Verifier",
		},
		{
			name:       "hmac_sha1 returns HMACSha1Verifier",
			verifyType: "hmac_sha1",
			wantType:   "*verify.HMACSha1Verifier",
		},
		{
			name:       "basic_auth returns BasicAuthVerifier",
			verifyType: "basic_auth",
			wantType:   "*verify.BasicAuthVerifier",
		},
		{
			name:       "bearer_token returns BearerTokenVerifier",
			verifyType: "bearer_token",
			wantType:   "*verify.BearerTokenVerifier",
		},
		{
			name:       "none returns NoopVerifier",
			verifyType: "none",
			wantType:   "*verify.NoopVerifier",
		},
		{
			name:       "unknown type returns error",
			verifyType: "unknown_type",
			wantErr:    true,
		},
		{
			name:       "empty string returns error",
			verifyType: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := GetVerifier(tt.verifyType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for type %q, got nil", tt.verifyType)
				}
				if v != nil {
					t.Fatalf("expected nil verifier on error, got %T", v)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := fmt.Sprintf("%T", v)
			if got != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helper: compute HMAC-SHA256 hex digest
// ---------------------------------------------------------------------------

func computeHMACSHA256(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// helper: compute HMAC-SHA1 hex digest
// ---------------------------------------------------------------------------

func computeHMACSHA1(secret, payload string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// HMACSha256Verifier
// ---------------------------------------------------------------------------

func TestHMACSha256Verifier(t *testing.T) {
	secret := "test-webhook-secret"
	body := []byte(`{"event":"push","repo":"hookrelay"}`)

	// Pre-compute valid signatures for the simple format.
	validHexSig := computeHMACSHA256(secret, string(body))
	validPrefixedSig := "sha256=" + validHexSig

	// Pre-compute valid Stripe-format signature.
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	stripePayload := ts + "." + string(body)
	validStripeSig := computeHMACSHA256(secret, stripePayload)
	stripeHeaderValue := fmt.Sprintf("t=%s,v1=%s", ts, validStripeSig)

	// Expired timestamp for tolerance test (1 hour ago).
	oldTs := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	oldStripePayload := oldTs + "." + string(body)
	oldStripeSig := computeHMACSHA256(secret, oldStripePayload)
	oldStripeHeaderValue := fmt.Sprintf("t=%s,v1=%s", oldTs, oldStripeSig)

	tests := []struct {
		name    string
		config  map[string]any
		headers map[string]string
		body    []byte
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid simple signature with sha256= prefix",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature-256",
			},
			headers: map[string]string{
				"x-hub-signature-256": validPrefixedSig,
			},
			body:    body,
			wantErr: false,
		},
		{
			name: "valid raw hex signature (no prefix)",
			config: map[string]any{
				"secret": secret,
				"header": "X-Signature",
			},
			headers: map[string]string{
				"x-signature": validHexSig,
			},
			body:    body,
			wantErr: false,
		},
		{
			name: "valid Stripe format signature",
			config: map[string]any{
				"secret": secret,
				"header": "Stripe-Signature",
			},
			headers: map[string]string{
				"stripe-signature": stripeHeaderValue,
			},
			body:    body,
			wantErr: false,
		},
		{
			name: "Stripe format with tolerance - within tolerance",
			config: map[string]any{
				"secret":    secret,
				"header":    "Stripe-Signature",
				"tolerance": float64(600), // 10 minutes
			},
			headers: map[string]string{
				"stripe-signature": stripeHeaderValue,
			},
			body:    body,
			wantErr: false,
		},
		{
			name: "Stripe format with tolerance - expired timestamp",
			config: map[string]any{
				"secret":    secret,
				"header":    "Stripe-Signature",
				"tolerance": float64(300), // 5 minutes; signature is 1 hour old
			},
			headers: map[string]string{
				"stripe-signature": oldStripeHeaderValue,
			},
			body:    body,
			wantErr: true,
			errMsg:  "signature timestamp outside tolerance",
		},
		{
			name: "invalid signature fails",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature-256",
			},
			headers: map[string]string{
				"x-hub-signature-256": "sha256=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			},
			body:    body,
			wantErr: true,
			errMsg:  "HMAC-SHA256 signature mismatch",
		},
		{
			name: "missing header fails",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature-256",
			},
			headers: map[string]string{},
			body:    body,
			wantErr: true,
			errMsg:  "missing signature header",
		},
		{
			name: "missing secret config fails",
			config: map[string]any{
				"header": "X-Hub-Signature-256",
			},
			headers: map[string]string{
				"x-hub-signature-256": validPrefixedSig,
			},
			body:    body,
			wantErr: true,
			errMsg:  "missing config key: secret",
		},
		{
			name: "missing header config fails",
			config: map[string]any{
				"secret": secret,
			},
			headers: map[string]string{
				"x-hub-signature-256": validPrefixedSig,
			},
			body:    body,
			wantErr: true,
			errMsg:  "missing config key: header",
		},
		{
			name: "Stripe format with invalid signature fails",
			config: map[string]any{
				"secret": secret,
				"header": "Stripe-Signature",
			},
			headers: map[string]string{
				"stripe-signature": fmt.Sprintf("t=%s,v1=bad_hex_sig", ts),
			},
			body:    body,
			wantErr: true,
			errMsg:  "HMAC-SHA256 signature mismatch (Stripe format)",
		},
	}

	v := &HMACSha256Verifier{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Verify(tt.config, tt.headers, tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" {
					if got := err.Error(); !contains(got, tt.errMsg) {
						t.Errorf("error %q does not contain %q", got, tt.errMsg)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HMACSha1Verifier
// ---------------------------------------------------------------------------

func TestHMACSha1Verifier(t *testing.T) {
	secret := "github-webhook-secret"
	body := []byte(`{"action":"completed","workflow_run":{"id":123}}`)

	validSig := "sha1=" + computeHMACSHA1(secret, string(body))

	tests := []struct {
		name    string
		config  map[string]any
		headers map[string]string
		body    []byte
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid GitHub-style sha1 signature",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature",
			},
			headers: map[string]string{
				"x-hub-signature": validSig,
			},
			body:    body,
			wantErr: false,
		},
		{
			name: "invalid signature fails",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature",
			},
			headers: map[string]string{
				"x-hub-signature": "sha1=0000000000000000000000000000000000000000",
			},
			body:    body,
			wantErr: true,
			errMsg:  "HMAC-SHA1 signature mismatch",
		},
		{
			name: "missing sha1= prefix fails",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature",
			},
			headers: map[string]string{
				"x-hub-signature": computeHMACSHA1(secret, string(body)),
			},
			body:    body,
			wantErr: true,
			errMsg:  "HMAC-SHA1 signature must start with 'sha1='",
		},
		{
			name: "missing header fails",
			config: map[string]any{
				"secret": secret,
				"header": "X-Hub-Signature",
			},
			headers: map[string]string{},
			body:    body,
			wantErr: true,
			errMsg:  "missing signature header",
		},
		{
			name: "missing secret config fails",
			config: map[string]any{
				"header": "X-Hub-Signature",
			},
			headers: map[string]string{
				"x-hub-signature": validSig,
			},
			body:    body,
			wantErr: true,
			errMsg:  "missing config key: secret",
		},
		{
			name: "missing header config fails",
			config: map[string]any{
				"secret": secret,
			},
			headers: map[string]string{
				"x-hub-signature": validSig,
			},
			body:    body,
			wantErr: true,
			errMsg:  "missing config key: header",
		},
	}

	v := &HMACSha1Verifier{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Verify(tt.config, tt.headers, tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" {
					if got := err.Error(); !contains(got, tt.errMsg) {
						t.Errorf("error %q does not contain %q", got, tt.errMsg)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BasicAuthVerifier
// ---------------------------------------------------------------------------

func TestBasicAuthVerifier(t *testing.T) {
	username := "webhook-user"
	password := "s3cret-p@ss"
	validEncoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	tests := []struct {
		name    string
		config  map[string]any
		headers map[string]string
		body    []byte
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid credentials pass",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Basic " + validEncoded,
			},
			body:    []byte(`{}`),
			wantErr: false,
		},
		{
			name: "invalid password fails",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":wrong-password")),
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "Basic Auth credentials mismatch",
		},
		{
			name: "invalid username fails",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("wrong-user:"+password)),
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "Basic Auth credentials mismatch",
		},
		{
			name: "missing Authorization header fails",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "missing Authorization header",
		},
		{
			name: "non-Basic scheme fails",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Bearer some-token",
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "Authorization header must start with 'Basic '",
		},
		{
			name: "invalid base64 fails",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Basic !!!not-base64!!!",
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "invalid base64 in Authorization header",
		},
		{
			name: "missing colon in decoded value fails",
			config: map[string]any{
				"username": username,
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("nocolonhere")),
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "invalid Basic Auth format",
		},
		{
			name: "missing username config fails",
			config: map[string]any{
				"password": password,
			},
			headers: map[string]string{
				"authorization": "Basic " + validEncoded,
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "missing config key: username",
		},
		{
			name: "missing password config fails",
			config: map[string]any{
				"username": username,
			},
			headers: map[string]string{
				"authorization": "Basic " + validEncoded,
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "missing config key: password",
		},
	}

	v := &BasicAuthVerifier{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Verify(tt.config, tt.headers, tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" {
					if got := err.Error(); !contains(got, tt.errMsg) {
						t.Errorf("error %q does not contain %q", got, tt.errMsg)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BearerTokenVerifier
// ---------------------------------------------------------------------------

func TestBearerTokenVerifier(t *testing.T) {
	token := "my-super-secret-token"

	tests := []struct {
		name    string
		config  map[string]any
		headers map[string]string
		body    []byte
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid token passes (default authorization header)",
			config: map[string]any{
				"token": token,
			},
			headers: map[string]string{
				"authorization": "Bearer " + token,
			},
			body:    []byte(`{}`),
			wantErr: false,
		},
		{
			name: "invalid token fails",
			config: map[string]any{
				"token": token,
			},
			headers: map[string]string{
				"authorization": "Bearer wrong-token",
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "bearer token mismatch",
		},
		{
			name: "custom header name works",
			config: map[string]any{
				"token":  token,
				"header": "X-Webhook-Token",
			},
			headers: map[string]string{
				"x-webhook-token": "Bearer " + token,
			},
			body:    []byte(`{}`),
			wantErr: false,
		},
		{
			name: "custom header name - missing header fails",
			config: map[string]any{
				"token":  token,
				"header": "X-Webhook-Token",
			},
			headers: map[string]string{
				"authorization": "Bearer " + token, // wrong header present
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "missing header: x-webhook-token",
		},
		{
			name: "missing default authorization header fails",
			config: map[string]any{
				"token": token,
			},
			headers: map[string]string{},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "missing header: authorization",
		},
		{
			name:   "missing token config fails",
			config: map[string]any{},
			headers: map[string]string{
				"authorization": "Bearer " + token,
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "missing config key: token",
		},
		{
			name: "wrong scheme (Basic instead of Bearer) fails",
			config: map[string]any{
				"token": token,
			},
			headers: map[string]string{
				"authorization": "Basic " + token,
			},
			body:    []byte(`{}`),
			wantErr: true,
			errMsg:  "bearer token mismatch",
		},
	}

	v := &BearerTokenVerifier{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Verify(tt.config, tt.headers, tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" {
					if got := err.Error(); !contains(got, tt.errMsg) {
						t.Errorf("error %q does not contain %q", got, tt.errMsg)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NoopVerifier
// ---------------------------------------------------------------------------

func TestNoopVerifier(t *testing.T) {
	v := &NoopVerifier{}

	tests := []struct {
		name    string
		config  map[string]any
		headers map[string]string
		body    []byte
	}{
		{
			name:    "nil config, nil headers, nil body",
			config:  nil,
			headers: nil,
			body:    nil,
		},
		{
			name:    "empty maps and body",
			config:  map[string]any{},
			headers: map[string]string{},
			body:    []byte{},
		},
		{
			name: "populated config and headers",
			config: map[string]any{
				"anything": "goes",
			},
			headers: map[string]string{
				"x-custom": "value",
			},
			body: []byte(`{"data":"payload"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v.Verify(tt.config, tt.headers, tt.body); err != nil {
				t.Fatalf("NoopVerifier should always return nil, got: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// configString / configFloat helpers (exported only inside the package)
// ---------------------------------------------------------------------------

func TestConfigString(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		key     string
		want    string
		wantErr bool
	}{
		{
			name:   "existing string key",
			config: map[string]any{"key": "value"},
			key:    "key",
			want:   "value",
		},
		{
			name:    "missing key",
			config:  map[string]any{},
			key:     "missing",
			wantErr: true,
		},
		{
			name:    "non-string value",
			config:  map[string]any{"key": 42},
			key:     "key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configString(tt.config, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigFloat(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]any
		key    string
		want   float64
		wantOk bool
	}{
		{
			name:   "existing float key",
			config: map[string]any{"tol": float64(300)},
			key:    "tol",
			want:   300,
			wantOk: true,
		},
		{
			name:   "missing key",
			config: map[string]any{},
			key:    "missing",
			want:   0,
			wantOk: false,
		},
		{
			name:   "non-float value",
			config: map[string]any{"key": "not-a-float"},
			key:    "key",
			want:   0,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := configFloat(tt.config, tt.key)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// contains is a small helper to check substring presence.
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
