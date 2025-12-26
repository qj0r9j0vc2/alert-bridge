package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// SignatureVersion is the Slack signature version prefix
	SignatureVersion = "v0"

	// MaxTimestampAge is the maximum age of a request timestamp (5 minutes)
	MaxTimestampAge = 5 * time.Minute
)

// SignatureVerifier provides Slack request signature verification.
type SignatureVerifier struct {
	signingSecret string
}

// NewSignatureVerifier creates a new signature verifier.
func NewSignatureVerifier(signingSecret string) *SignatureVerifier {
	return &SignatureVerifier{
		signingSecret: signingSecret,
	}
}

// VerifySignature verifies a Slack request signature using HMAC-SHA256.
// Per Slack spec: https://api.slack.com/authentication/verifying-requests-from-slack
//
// Parameters:
//   - timestamp: X-Slack-Request-Timestamp header value (Unix timestamp)
//   - body: Raw request body (must not be parsed before verification)
//   - signature: X-Slack-Signature header value (format: "v0=<hex_signature>")
//
// Returns error if:
//   - Timestamp is too old (>5 minutes)
//   - Signature format is invalid
//   - Computed signature doesn't match provided signature
func (v *SignatureVerifier) VerifySignature(timestamp string, body []byte, signature string) error {
	// Validate timestamp freshness (prevent replay attacks)
	if err := v.validateTimestamp(timestamp); err != nil {
		return err
	}

	// Validate signature format
	if !strings.HasPrefix(signature, SignatureVersion+"=") {
		return fmt.Errorf("invalid signature format: expected prefix '%s='", SignatureVersion)
	}

	// Extract hex signature
	providedSig := strings.TrimPrefix(signature, SignatureVersion+"=")

	// Compute expected signature
	// Signature base string: v0:<timestamp>:<body>
	baseString := fmt.Sprintf("%s:%s:%s", SignatureVersion, timestamp, string(body))
	expectedSig := v.computeSignature(baseString)

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(expectedSig), []byte(providedSig)) {
		return fmt.Errorf("signature mismatch: request may be forged or tampered")
	}

	return nil
}

// validateTimestamp checks if the request timestamp is within acceptable range.
// Rejects requests older than 5 minutes to prevent replay attacks.
func (v *SignatureVerifier) validateTimestamp(timestamp string) error {
	// Parse timestamp
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	// Convert to time.Time
	requestTime := time.Unix(ts, 0)
	now := time.Now()

	// Check if timestamp is in the future (clock skew tolerance: 1 minute)
	if requestTime.After(now.Add(1 * time.Minute)) {
		return fmt.Errorf("timestamp is in the future: %s (current: %s)",
			requestTime.Format(time.RFC3339),
			now.Format(time.RFC3339))
	}

	// Check if timestamp is too old
	age := now.Sub(requestTime)
	if age > MaxTimestampAge {
		return fmt.Errorf("timestamp too old: %s (age: %s, max: %s)",
			requestTime.Format(time.RFC3339),
			age.String(),
			MaxTimestampAge.String())
	}

	return nil
}

// computeSignature computes HMAC-SHA256 signature for the given base string.
func (v *SignatureVerifier) computeSignature(baseString string) string {
	h := hmac.New(sha256.New, []byte(v.signingSecret))
	h.Write([]byte(baseString))
	return hex.EncodeToString(h.Sum(nil))
}
