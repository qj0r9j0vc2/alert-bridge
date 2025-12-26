package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	// ServiceName is the name of this service for observability
	ServiceName = "alert-bridge"
)

// Telemetry holds the OpenTelemetry providers for tracing and metrics.
type Telemetry struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Metrics        *Metrics
}

// NewTelemetry creates and initializes OpenTelemetry telemetry.
// For now, we use:
// - NoOp tracer (can be extended to OTLP exporter later)
// - Prometheus metrics exporter
func NewTelemetry(serviceName, serviceVersion string) (*Telemetry, error) {
	if serviceName == "" {
		serviceName = ServiceName
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	// Setup metrics with Prometheus exporter
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(prometheusExporter),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Create metrics
	metrics, err := NewMetrics(meterProvider.Meter(serviceName))
	if err != nil {
		return nil, fmt.Errorf("creating metrics: %w", err)
	}

	// For now, use NoOp tracer (can be extended to OTLP later)
	tracerProvider := noop.NewTracerProvider()

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	return &Telemetry{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		Metrics:        metrics,
	}, nil
}

// Shutdown cleanly shuts down the telemetry providers.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if mp, ok := t.MeterProvider.(*sdkmetric.MeterProvider); ok {
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down meter provider: %w", err)
		}
	}
	return nil
}
