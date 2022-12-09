package main

import (
	"context"

	propagators_b3 "go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

func InitMeterProvider(lc fx.Lifecycle) (metric.MeterProvider, *otelprom.Exporter, error) {
	exporter, err := otelprom.New()
	if err != nil {
		return nil, nil, err
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return provider.Shutdown(ctx)
		},
	})

	return provider, exporter, nil
}

func InitTracerProvider(lc fx.Lifecycle) (trace.TracerProvider, error) {
	var tp *sdktrace.TracerProvider

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient())
			if err != nil {
				return err
			}

			tp = sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithBatcher(exporter),
				sdktrace.WithResource(resource.NewWithAttributes(
					semconv.SchemaURL,
					semconv.ServiceNameKey.String("jn"),
					semconv.ServiceVersionKey.String("0.0.1"),
				)),
			)

			otel.SetTracerProvider(tp)
			otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}, propagators_b3.New()))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	})

	return otel.GetTracerProvider(), nil
}
