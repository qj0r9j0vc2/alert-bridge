package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// WebhookSecretGetter is a function that returns the current webhook secret.
// Used for hot-reload support - secret is read on each request.
type WebhookSecretGetter func() string

// PagerDutyAuth creates middleware for PagerDuty webhook signature verification.
// Implements the PagerDuty webhook signature verification protocol:
// https://developer.pagerduty.com/docs/ZG9jOjQ1MTg4ODQ0-verifying-signatures
//
// The secretGetter function is called on each request to support configuration hot-reload.
func PagerDutyAuth(secretGetter WebhookSecretGetter, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read current secret (supports hot-reload)
			webhookSecret := secretGetter()

			// Skip auth if no secret configured (backward compatible)
			if webhookSecret == "" {
				logger.Warn("pagerduty webhook secret not configured, skipping signature verification")
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

			// Get signatures from header (PagerDuty may send multiple)
			signatures := r.Header.Values("X-PagerDuty-Signature")
			if len(signatures) == 0 {
				logger.Warn("pagerduty webhook signature validation failed",
					"reason", "missing_signature",
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
				)
				http.Error(w, "missing signature", http.StatusUnauthorized)
				return
			}

			// Verify at least one signature matches
			if !verifyPagerDutySignature(body, signatures, webhookSecret) {
				logger.Warn("pagerduty webhook signature validation failed",
					"reason", "invalid_signature",
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent(),
					"signature_count", len(signatures),
				)
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}

			// Log successful validation for security audit trail
			logger.Info("pagerduty webhook signature validated",
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			// Restore body for handler
			r.Body = io.NopCloser(bytes.NewReader(body))

			next.ServeHTTP(w, r)
		})
	}
}

// verifyPagerDutySignature verifies the PagerDuty webhook signature.
// PagerDuty sends multiple signatures with different versions.
// Returns true if at least one signature matches.
func verifyPagerDutySignature(body []byte, signatures []string, webhookSecret string) bool {
	for _, sig := range signatures {
		// Parse version and signature
		// Format: "v1=<signature>"
		parts := strings.SplitN(sig, "=", 2)
		if len(parts) != 2 {
			continue
		}

		version := parts[0]
		signature := parts[1]

		// Only support v1 signatures
		if version != "v1" {
			continue
		}

		// Compute expected signature
		mac := hmac.New(sha256.New, []byte(webhookSecret))
		mac.Write(body)
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		// Constant-time comparison to prevent timing attacks
		if hmac.Equal([]byte(signature), []byte(expectedSig)) {
			return true
		}
	}

	return false
}
