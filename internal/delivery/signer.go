package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// sign produces an outbound HMAC-SHA256 signature in the format:
// X-HookRelay-Signature: t=<timestamp>,v1=<hex>
func sign(secret, timestamp string, body []byte) string {
	payload := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%s,v1=%s", timestamp, sig)
}
