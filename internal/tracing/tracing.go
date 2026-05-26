package tracing

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const ServiceName = "image-optimize-proxy"

// Init initializes the OTel tracing provider if OTEL_EXPORTER_OTLP_ENDPOINT is set.
// Returns a shutdown function (always non-nil) and a boolean indicating if tracing is enabled.
// If the endpoint is not set, returns a no-op shutdown and false.
func Init(ctx context.Context) (shutdown func(context.Context) error, enabled bool, err error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		return func(context.Context) error { return nil }, false, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return func(context.Context) error { return nil }, false, err
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(ServiceName),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, true, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer() trace.Tracer {
	return otel.Tracer(ServiceName)
}
