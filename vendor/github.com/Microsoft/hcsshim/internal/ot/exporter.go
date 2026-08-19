package ot

import (
	"context"
	"maps"

	"github.com/sirupsen/logrus"

	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/logfields"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const spanMessage = "Span"

var _errorCodeKey = logrus.ErrorKey + "Code"

// LogrusExporter is an OpenTelemetry `sdktrace.SpanExporter` that exports
// `trace.ReadOnlySpan` to logrus output.
type LogrusExporter struct{}

var _ sdktrace.SpanExporter = &LogrusExporter{}

// ExportSpans exports each `spans` based on the following rules:
//
// 1. All output will contain `s.Attributes`, `s.SpanKind`, `s.TraceID`,
// `s.SpanID`, and `s.ParentSpanID` for correlation
//
// 2. Any calls to .Annotate will not be supported.
//
// 3. The span itself will be written at `logrus.InfoLevel` unless
// `s.Status.Code != 0` in which case it will be written at `logrus.ErrorLevel`
// providing `s.Status.Message` as the error value.
func (le *LogrusExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, s := range spans {
		if s.DroppedAttributes() > 0 {
			logrus.WithFields(logrus.Fields{
				"name":            s.Name(),
				logfields.TraceID: s.SpanContext().TraceID().String(),
				logfields.SpanID:  s.SpanContext().SpanID().String(),
				"dropped":         s.DroppedAttributes(),
				"maxAttributes":   len(s.Attributes()),
			}).Warning("span had dropped attributes")
		}

		entry := log.L.Dup()
		// Combine all span annotations with span data (eg, trace ID, span ID, parent span ID,
		// error, status code)
		// Span attributes are guaranteed to be strings, bools, or int64s, so we can
		// skip overhead in entry.WithFields() and add them directly to entry.Data.
		// Preallocate ahead of time, since we should add, at most, 10 additional entries
		data := make(logrus.Fields, len(entry.Data)+len(s.Attributes())+10)

		// Default log entry may have prexisting/application-wide data
		maps.Copy(data, entry.Data)
		for _, attr := range s.Attributes() {
			data[string(attr.Key)] = attr.Value.AsInterface()
		}

		sc := s.SpanContext()
		data[logfields.Name] = s.Name()
		data[logfields.TraceID] = sc.TraceID().String()
		data[logfields.SpanID] = sc.SpanID().String()
		if s.Parent().IsValid() {
			data[logfields.ParentSpanID] = s.Parent().SpanID().String()
		}
		data[logfields.StartTime] = s.StartTime()
		data[logfields.EndTime] = s.EndTime()
		data[logfields.Duration] = s.EndTime().Sub(s.StartTime())
		if sk := spanKindToString(s.SpanKind()); sk != "" {
			data["spanKind"] = sk
		}

		level := logrus.InfoLevel
		if s.Status().Code == otelcodes.Error {
			level = logrus.ErrorLevel

			// don't overwrite an existing "error" or "errorCode" attributes
			if _, ok := data[logrus.ErrorKey]; !ok {
				data[logrus.ErrorKey] = s.Status().Description
			}
			if _, ok := data[_errorCodeKey]; !ok {
				data[_errorCodeKey] = s.Status().Code.String()
			}
		}

		entry.Data = data
		entry.Time = s.StartTime()
		entry.Log(level, spanMessage)
	}
	return nil
}

func (le *LogrusExporter) Shutdown(ctx context.Context) error {
	log.G(ctx).Trace("LogrusExporter shutting down")
	return nil
}
