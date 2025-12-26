package handler

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler serves Prometheus metrics.
type MetricsHandler struct {
	handler http.Handler
}

// NewMetricsHandler creates a new metrics handler.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		handler: promhttp.Handler(),
	}
}

// ServeHTTP handles GET /metrics
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.handler.ServeHTTP(w, r)
}
