package middleware

import (
	"net/http"
	"time"

	"github.com/qj0r9j0vc2/alert-bridge/internal/infrastructure/observability"
)

// Observability records HTTP metrics for requests.
func Observability(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Increment active requests
			metrics.HTTPRequestsActive.Add(r.Context(), 1)
			defer metrics.HTTPRequestsActive.Add(r.Context(), -1)

			// Wrap response writer to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			metrics.RecordHTTPRequest(
				r.Context(),
				r.Method,
				r.URL.Path,
				rw.statusCode,
				duration,
			)
		})
	}
}
