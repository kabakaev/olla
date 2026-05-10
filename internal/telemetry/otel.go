package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/thushan/olla/internal/config"
	"github.com/thushan/olla/internal/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
}

func (p *Providers) Shutdown(ctx context.Context) error {
	var errs []error
	if p == nil {
		return nil
	}
	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if p.LoggerProvider != nil {
		if err := p.LoggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("telemetry shutdown failed: %v", errs)
}

func InitProviders(ctx context.Context, cfg config.TelemetryConfig) (*Providers, error) {
	if !cfg.Enabled {
		InitMetrics(otel.GetMeterProvider().Meter("github.com/thushan/olla"))
		return &Providers{}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(versionOrConfig(cfg.ServiceVersion)),
			attribute.String("service.instance.id", "olla"),
		),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithOS(),
	)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	providers := &Providers{}

	if cfg.Traces.Enabled {
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLP.Endpoint)}
		if cfg.OTLP.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create trace exporter: %w", err)
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
		providers.TracerProvider = tp
	}

	if cfg.Metrics.Enabled {
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.OTLP.Endpoint)}
		if cfg.OTLP.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		exporter, err := otlpmetricgrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create metric exporter: %w", err)
		}

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second))),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		providers.MeterProvider = mp
		InitMetrics(mp.Meter("github.com/thushan/olla"))
	} else {
		InitMetrics(otel.GetMeterProvider().Meter("github.com/thushan/olla"))
	}

	if cfg.Logs.Enabled {
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(cfg.OTLP.Endpoint)}
		if cfg.OTLP.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		exporter, err := otlploggrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("create log exporter: %w", err)
		}

		lp := sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
			sdklog.WithResource(res),
		)
		providers.LoggerProvider = lp
	}

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return providers, nil
}

func Tracer() trace.Tracer {
	return otel.Tracer("github.com/thushan/olla")
}

func Meter() metric.Meter {
	return otel.Meter("github.com/thushan/olla")
}

func LoggerProvider(providers *Providers) otellog.LoggerProvider {
	if providers == nil || providers.LoggerProvider == nil {
		return nil
	}
	return providers.LoggerProvider
}

func versionOrConfig(v string) string {
	if v != "" {
		return v
	}
	return version.Version
}
