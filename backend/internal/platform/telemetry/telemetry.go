package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"os"
)

// Structured logging and instrumentation are always available. An OTLP exporter
// can be installed at the process boundary without changing application services.
func Init(service string) *slog.Logger {
	l := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", service)
	slog.SetDefault(l)
	return l
}

func Tracer() trace.Tracer { return otel.Tracer("saveboard") }
