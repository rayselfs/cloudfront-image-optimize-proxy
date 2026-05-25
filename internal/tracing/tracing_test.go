package tracing

import (
	"context"
	"testing"
)

func TestInitNoEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, enabled, err := Init(context.Background())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if enabled {
		t.Fatal("Init() enabled = true, want false")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestInitWithEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")

	shutdown, enabled, err := Init(context.Background())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if !enabled {
		t.Fatal("Init() enabled = false, want true")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}
