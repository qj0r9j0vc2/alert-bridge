package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Timeout creates middleware that sets a timeout for request processing.
// If the request exceeds the timeout, it returns 504 Gateway Timeout.
// Excludes /metrics, /health, and /ready endpoints from timeout.
func Timeout(timeout time.Duration, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip timeout for observability and health endpoints
			if r.URL.Path == "/metrics" || r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/" {
				next.ServeHTTP(w, r)
				return
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			// Channel to signal completion
			done := make(chan struct{})
			var responseWritten bool

			// Wrap response writer to track if response was written
			wrappedWriter := &timeoutResponseWriter{
				ResponseWriter: w,
				onWrite: func() {
					responseWritten = true
					close(done)
				},
			}

			// Run handler in goroutine
			go func() {
				next.ServeHTTP(wrappedWriter, r.WithContext(ctx))
				if !responseWritten {
					close(done)
				}
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				// Request completed successfully
				return
			case <-ctx.Done():
				// Request timed out
				if !responseWritten {
					logger.Warn("request timeout",
						"path", r.URL.Path,
						"method", r.Method,
						"timeout", timeout,
					)
					http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
				}
			}
		})
	}
}

// timeoutResponseWriter wraps http.ResponseWriter to track writes
type timeoutResponseWriter struct {
	http.ResponseWriter
	onWrite     func()
	wroteHeader bool
	statusCode  int
}

func (w *timeoutResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.statusCode = statusCode
		w.ResponseWriter.WriteHeader(statusCode)
		w.wroteHeader = true
		if w.onWrite != nil {
			w.onWrite()
			w.onWrite = nil
		}
	}
}

func (w *timeoutResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
