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

// AlertmanagerAuth creates middleware for Alertmanager webhook authentication.
// If secret is empty, authentication is skipped (backward compatible).
//
// Expected header format: X-Alertmanager-Signature: v1=<hex_hmac_sha256>
func AlertmanagerAuth(secret string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if no secret configured (backward compatible)
			if secret == "" {
				next.ServeHTTP(w, r)
				return
			}

			signature := r.Header.Get("X-Alertmanager-Signature")
			if signature == "" {
				logger.Warn("missing alertmanager signature header",
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
				)
				http.Error(w, "missing signature", http.StatusUnauthorized)
				return
			}

			// Read body for signature verification
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Error("failed to read request body",
					"error", err,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			r.Body.Close()

			// Verify signature
			if !verifyAlertmanagerSignature(body, signature, secret) {
				logger.Warn("invalid alertmanager signature",
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
				)
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}

			// Restore body for handler
			r.Body = io.NopCloser(bytes.NewReader(body))

			logger.Debug("alertmanager webhook authenticated",
				"remote_addr", r.RemoteAddr,
			)

			next.ServeHTTP(w, r)
		})
	}
}

// verifyAlertmanagerSignature validates HMAC-SHA256 signature.
// Expected format: "v1=<hex_signature>"
func verifyAlertmanagerSignature(body []byte, signature, secret string) bool {
	// Parse signature format: "v1=<hex_signature>"
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "v1" {
		return false
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(parts[1]), []byte(expectedSig))
}
