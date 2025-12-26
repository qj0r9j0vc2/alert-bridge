package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds all application metrics.
type Metrics struct {
	meter metric.Meter

	// HTTP metrics
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram
	HTTPRequestsActive  metric.Int64UpDownCounter

	// Alert processing metrics
	AlertsProcessedTotal metric.Int64Counter
	AlertProcessingDuration metric.Float64Histogram
	AlertsActiveGauge    metric.Int64UpDownCounter

	// Notification metrics
	NotificationsSentTotal   metric.Int64Counter
	NotificationDuration     metric.Float64Histogram
	NotificationRetriesTotal metric.Int64Counter
	NotificationErrorsTotal  metric.Int64Counter

	// Acknowledgment metrics
	AcknowledgmentsSyncedTotal metric.Int64Counter
	AcknowledgmentErrorsTotal  metric.Int64Counter

	// Repository metrics
	RepositoryOperationsTotal   metric.Int64Counter
	RepositoryOperationDuration metric.Float64Histogram
}

// NewMetrics creates and registers all application metrics.
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	m := &Metrics{meter: meter}

	var err error

	// HTTP metrics
	m.HTTPRequestsTotal, err = meter.Int64Counter(
		"http.server.requests.total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating http_requests_total: %w", err)
	}

	m.HTTPRequestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating http_request_duration: %w", err)
	}

	m.HTTPRequestsActive, err = meter.Int64UpDownCounter(
		"http.server.requests.active",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{requests}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating http_requests_active: %w", err)
	}

	// Alert processing metrics
	m.AlertsProcessedTotal, err = meter.Int64Counter(
		"alerts.processed.total",
		metric.WithDescription("Total number of alerts processed"),
		metric.WithUnit("{alerts}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating alerts_processed_total: %w", err)
	}

	m.AlertProcessingDuration, err = meter.Float64Histogram(
		"alerts.processing.duration",
		metric.WithDescription("Alert processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating alert_processing_duration: %w", err)
	}

	m.AlertsActiveGauge, err = meter.Int64UpDownCounter(
		"alerts.active",
		metric.WithDescription("Number of active alerts"),
		metric.WithUnit("{alerts}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating alerts_active: %w", err)
	}

	// Notification metrics
	m.NotificationsSentTotal, err = meter.Int64Counter(
		"notifications.sent.total",
		metric.WithDescription("Total number of notifications sent"),
		metric.WithUnit("{notifications}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating notifications_sent_total: %w", err)
	}

	m.NotificationDuration, err = meter.Float64Histogram(
		"notifications.send.duration",
		metric.WithDescription("Notification send duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating notification_duration: %w", err)
	}

	m.NotificationRetriesTotal, err = meter.Int64Counter(
		"notifications.retries.total",
		metric.WithDescription("Total number of notification retries"),
		metric.WithUnit("{retries}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating notification_retries_total: %w", err)
	}

	m.NotificationErrorsTotal, err = meter.Int64Counter(
		"notifications.errors.total",
		metric.WithDescription("Total number of notification errors"),
		metric.WithUnit("{errors}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating notification_errors_total: %w", err)
	}

	// Acknowledgment metrics
	m.AcknowledgmentsSyncedTotal, err = meter.Int64Counter(
		"acknowledgments.synced.total",
		metric.WithDescription("Total number of acknowledgments synced"),
		metric.WithUnit("{acks}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating acknowledgments_synced_total: %w", err)
	}

	m.AcknowledgmentErrorsTotal, err = meter.Int64Counter(
		"acknowledgments.errors.total",
		metric.WithDescription("Total number of acknowledgment errors"),
		metric.WithUnit("{errors}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating acknowledgment_errors_total: %w", err)
	}

	// Repository metrics
	m.RepositoryOperationsTotal, err = meter.Int64Counter(
		"repository.operations.total",
		metric.WithDescription("Total number of repository operations"),
		metric.WithUnit("{operations}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating repository_operations_total: %w", err)
	}

	m.RepositoryOperationDuration, err = meter.Float64Histogram(
		"repository.operation.duration",
		metric.WithDescription("Repository operation duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating repository_operation_duration: %w", err)
	}

	return m, nil
}

// RecordHTTPRequest records HTTP request metrics.
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.route", path),
		attribute.Int("http.status_code", statusCode),
	}

	m.HTTPRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.HTTPRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordAlertProcessed records alert processing metrics.
func (m *Metrics) RecordAlertProcessed(ctx context.Context, alertName, severity, status string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("alert.name", alertName),
		attribute.String("alert.severity", severity),
		attribute.String("status", status),
		attribute.Bool("success", success),
	}

	m.AlertsProcessedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.AlertProcessingDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordNotificationSent records notification metrics.
func (m *Metrics) RecordNotificationSent(ctx context.Context, notifier string, success bool, duration time.Duration, retries int) {
	attrs := []attribute.KeyValue{
		attribute.String("notifier", notifier),
		attribute.Bool("success", success),
	}

	m.NotificationsSentTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.NotificationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if retries > 0 {
		m.NotificationRetriesTotal.Add(ctx, int64(retries), metric.WithAttributes(attrs...))
	}

	if !success {
		m.NotificationErrorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordAcknowledgmentSynced records acknowledgment sync metrics.
func (m *Metrics) RecordAcknowledgmentSynced(ctx context.Context, source string, syncedSystems int, errors int) {
	attrs := []attribute.KeyValue{
		attribute.String("source", source),
	}

	m.AcknowledgmentsSyncedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	if errors > 0 {
		errAttrs := append(attrs, attribute.Int("synced_systems", syncedSystems))
		m.AcknowledgmentErrorsTotal.Add(ctx, int64(errors), metric.WithAttributes(errAttrs...))
	}
}

// RecordRepositoryOperation records repository operation metrics.
func (m *Metrics) RecordRepositoryOperation(ctx context.Context, operation, entity string, duration time.Duration, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.String("entity", entity),
		attribute.Bool("success", success),
	}

	m.RepositoryOperationsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.RepositoryOperationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
