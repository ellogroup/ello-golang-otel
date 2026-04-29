package provider

import (
	"context"
	"fmt"
	"github.com/ellogroup/ello-golang-otel/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewMeterProvider creates and registers a global MeterProvider configured from cfg.
//
// Returns:
//   - A metric.Meter scoped to cfg.ServiceName, ready for creating instruments.
//   - A shutdown function that flushes pending metrics; call it on Lambda shutdown.
//
// When cfg.Enabled is false a no-op meter is returned with zero overhead.
func NewMeterProvider(ctx context.Context, cfg config.Config) (metric.Meter, func(context.Context) error, error) {
	if !cfg.Enabled {
		return metricnoop.NewMeterProvider().Meter(cfg.ServiceName), func(context.Context) error { return nil }, nil
	}

	exp, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.Endpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTEL resource: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp.Meter(cfg.ServiceName), mp.Shutdown, nil
}
