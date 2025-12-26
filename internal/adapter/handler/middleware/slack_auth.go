package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// SlackAuth creates middleware for Slack webhook signature verification.
// Implements the Slack signature verification protocol:
// https://api.slack.com/authentication/verifying-requests-from-slack
func SlackAuth(signingSecret string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no secret configured
			if signingSecret == "" {
				logger.Warn("slack signing secret not configured, skipping signature verification")
				next.ServeHTTP(w, r)
				return
			}

			// Read body for signature verification
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Error("failed to read request body", "error", err)
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			// Verify signature
			if err := verifySlackSignature(r.Header, body, signingSecret); err != nil {
				logger.Warn("invalid slack signature", "error", err)
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}

			// Restore body for handler
			r.Body = io.NopCloser(bytes.NewReader(body))

			next.ServeHTTP(w, r)
		})
	}
}

// verifySlackSignature verifies the Slack request signature.
func verifySlackSignature(header http.Header, body []byte, signingSecret string) error {
	timestamp := header.Get("X-Slack-Request-Timestamp")
	signature := header.Get("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return fmt.Errorf("missing timestamp or signature headers")
	}

	// Check timestamp is recent (within 5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	now := time.Now().Unix()
	if abs(now-ts) > 60*5 {
		return fmt.Errorf("timestamp too old (request age: %d seconds)", abs(now-ts))
	}

	// Compute expected signature
	// Format: v0:{timestamp}:{body}
	sigBaseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(sigBaseString))
	expectedSig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// abs returns the absolute value of an int64.
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
