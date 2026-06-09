// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package observ provides experimental observability instrumentation for the
// otlpmetrichttp exporter.
package observ // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/observ"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/x"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/semconv/v1.41.0/otelconv"
)

const (
	// ScopeName is the unique name of the meter used for instrumentation.
	ScopeName = "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp/internal/observ"

	// SchemaURL is the schema URL of the metrics produced by this
	// instrumentation.
	SchemaURL = semconv.SchemaURL

	// Version is the current version of this instrumentation.
	//
	// This matches the version of the exporter.
	Version = internal.Version
)

var (
	measureAttrsPool = &sync.Pool{
		New: func() any {
			const n = 1 + // component.name
				1 + // component.type
				1 + // server.addr
				1 + // server.port
				1 + // error.type
				1 // http.response.status_code
			s := make([]attribute.KeyValue, 0, n)
			// Return a pointer to a slice instead of a slice itself
			// to avoid allocations on every call.
			return &s
		},
	}

	addOptPool = &sync.Pool{
		New: func() any {
			const n = 1 // WithAttributeSet
			o := make([]metric.AddOption, 0, n)
			return &o
		},
	}

	recordOptPool = &sync.Pool{
		New: func() any {
			const n = 1 // WithAttributeSet
			o := make([]metric.RecordOption, 0, n)
			return &o
		},
	}
)

func get[T any](p *sync.Pool) *[]T { return p.Get().(*[]T) }

func put[T any](p *sync.Pool, s *[]T) {
	*s = (*s)[:0] // Reset.
	p.Put(s)
}

// ComponentName returns the component name for the exporter with the
// provided ID.
func ComponentName(id int64) string {
	t := semconv.OTelComponentTypeOtlpHTTPMetricExporter.Value.AsString()
	return fmt.Sprintf("%s/%d", t, id)
}

// Instrumentation is experimental instrumentation for the exporter.
type Instrumentation struct {
	inflightMetric metric.Int64UpDownCounter
	exportedMetric metric.Int64Counter
	opDuration     metric.Float64Histogram

	attrs  []attribute.KeyValue
	addOpt metric.AddOption
	recOpt metric.RecordOption
}

// NewInstrumentation returns instrumentation for an OTLP over HTTP metric
// exporter with the provided ID and endpoint. It uses the global
// MeterProvider to create the instrumentation.
//
// The id should be the unique exporter instance ID. It is used
// to set the "component.name" attribute.
//
// The endpoint is the HTTP endpoint the exporter is exporting to.
//
// If the experimental observability is disabled, nil is returned.
func NewInstrumentation(id int64, endpoint string) (*Instrumentation, error) {
	if !x.Observability.Enabled() {
		return nil, nil
	}

	attrs := BaseAttrs(id, endpoint)
	i := &Instrumentation{
		attrs:  attrs,
		addOpt: metric.WithAttributeSet(attribute.NewSet(attrs...)),

		// Do not modify attrs (NewSet sorts in-place), make a new slice.
		recOpt: metric.WithAttributeSet(attribute.NewSet(append(
			// Default to OK status code (200).
			[]attribute.KeyValue{semconv.HTTPResponseStatusCode(http.StatusOK)},
			attrs...,
		)...)),
	}

	mp := otel.GetMeterProvider()
	m := mp.Meter(
		ScopeName,
		metric.WithInstrumentationVersion(Version),
		metric.WithSchemaURL(SchemaURL),
	)

	var err error

	inflightMetric, e := otelconv.NewSDKExporterMetricDataPointInflight(m)
	if e != nil {
		e = fmt.Errorf("failed to create inflight metric: %w", e)
		err = errors.Join(err, e)
	}
	i.inflightMetric = inflightMetric.Inst()

	exportedMetric, e := otelconv.NewSDKExporterMetricDataPointExported(m)
	if e != nil {
		e = fmt.Errorf("failed to create exported metric: %w", e)
		err = errors.Join(err, e)
	}
	i.exportedMetric = exportedMetric.Inst()

	opDuration, e := otelconv.NewSDKExporterOperationDuration(m)
	if e != nil {
		e = fmt.Errorf("failed to create operation duration metric: %w", e)
		err = errors.Join(err, e)
	}
	i.opDuration = opDuration.Inst()

	return i, err
}

// BaseAttrs returns the base attributes for the exporter with the provided ID
// and endpoint.
//
// The id should be the unique exporter instance ID. It is used
// to set the "component.name" attribute.
//
// The endpoint is the HTTP endpoint the exporter is exporting to. It should be
// in the format "host[:port]".
func BaseAttrs(id int64, endpoint string) []attribute.KeyValue {
	host, port, err := parseEndpoint(endpoint)
	if err != nil || (host == "" && port < 0) {
		if err != nil {
			global.Debug("failed to parse endpoint", "endpoint", endpoint, "error", err)
		}
		return []attribute.KeyValue{
			semconv.OTelComponentName(ComponentName(id)),
			semconv.OTelComponentTypeOtlpHTTPMetricExporter,
		}
	}

	// Do not use append so the slice is exactly allocated.

	if port < 0 {
		return []attribute.KeyValue{
			semconv.OTelComponentName(ComponentName(id)),
			semconv.OTelComponentTypeOtlpHTTPMetricExporter,
			semconv.ServerAddress(host),
		}
	}

	if host == "" {
		return []attribute.KeyValue{
			semconv.OTelComponentName(ComponentName(id)),
			semconv.OTelComponentTypeOtlpHTTPMetricExporter,
			semconv.ServerPort(port),
		}
	}

	return []attribute.KeyValue{
		semconv.OTelComponentName(ComponentName(id)),
		semconv.OTelComponentTypeOtlpHTTPMetricExporter,
		semconv.ServerAddress(host),
		semconv.ServerPort(port),
	}
}

// parseEndpoint parses the host and port from endpoint that has the form
// "host[:port]", or it returns an error if the endpoint is not parsable.
//
// If no port is specified, -1 is returned.
//
// If no host is specified, an empty string is returned.
func parseEndpoint(endpoint string) (string, int, error) {
	// First check if the endpoint is just an IP address.
	if ip := parseIP(endpoint); ip != "" {
		return ip, -1, nil
	}

	// If there's no colon, there is no port (IPv6 with no port checked above).
	if !strings.Contains(endpoint, ":") {
		return endpoint, -1, nil
	}

	// Otherwise, parse as host:port.
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", -1, fmt.Errorf("invalid host:port %q: %w", endpoint, err)
	}

	const base, bitSize = 10, 16
	port16, err := strconv.ParseUint(portStr, base, bitSize)
	if err != nil {
		return "", -1, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	port := int(port16) // port is guaranteed to be in the range [0, 65535].

	return host, port, nil
}

// parseIP attempts to parse the entire endpoint as an IP address.
// It returns the normalized string form of the IP if successful,
// or an empty string if parsing fails.
func parseIP(ip string) string {
	// Strip leading and trailing brackets for IPv6 addresses.
	if len(ip) >= 2 && ip[0] == '[' && ip[len(ip)-1] == ']' {
		ip = ip[1 : len(ip)-1]
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	// Return the normalized string form of the IP.
	return addr.String()
}

// ExportMetrics instruments the UploadMetrics method of the client. It returns an
// [ExportOp] that must have its [ExportOp.End] method called when the
// operation ends.
func (i *Instrumentation) ExportMetrics(ctx context.Context, rm *metricpb.ResourceMetrics) ExportOp {
	start := time.Now()

	nMetrics := countDataPoints(rm)

	if i.inflightMetric.Enabled(ctx) {
		addOpt := get[metric.AddOption](addOptPool)
		defer put(addOptPool, addOpt)
		*addOpt = append(*addOpt, i.addOpt)
		i.inflightMetric.Add(ctx, nMetrics, *addOpt...)
	}

	return ExportOp{
		ctx:      ctx,
		start:    start,
		nMetrics: nMetrics,
		inst:     i,
	}
}

// ExportOp tracks the export operation being observed by
// [Instrumentation.ExportMetrics].
type ExportOp struct {
	ctx      context.Context
	start    time.Time
	nMetrics int64

	inst *Instrumentation
}

// End completes the observation of the operation being observed by a call to
// [Instrumentation.ExportMetrics].
//
// Any error that is encountered is provided as err.
// The HTTP status code from the response is provided as status.
//
// If err is not nil, all metrics will be recorded as failures unless error is of
// type [internal.PartialSuccess]. In the case of a PartialSuccess, the number
// of successfully exported metrics will be determined by inspecting the
// RejectedItems field of the PartialSuccess.
func (e ExportOp) End(err error, status int) {
	addOpt := get[metric.AddOption](addOptPool)
	defer put(addOptPool, addOpt)
	*addOpt = append(*addOpt, e.inst.addOpt)

	if e.inst.inflightMetric.Enabled(e.ctx) {
		e.inst.inflightMetric.Add(e.ctx, -e.nMetrics, *addOpt...)
	}

	success := successful(e.nMetrics, err)
	// Record successfully exported metrics, even if the value is 0 which are
	// meaningful to distribution aggregations.
	if e.inst.exportedMetric.Enabled(e.ctx) {
		e.inst.exportedMetric.Add(e.ctx, success, *addOpt...)
	}

	if err != nil && e.inst.exportedMetric.Enabled(e.ctx) {
		attrs := get[attribute.KeyValue](measureAttrsPool)
		defer put(measureAttrsPool, attrs)
		*attrs = append(*attrs, e.inst.attrs...)
		*attrs = append(*attrs, semconv.ErrorType(err))

		// Do not inefficiently make a copy of attrs by using
		// WithAttributes instead of WithAttributeSet.
		o := metric.WithAttributeSet(attribute.NewSet(*attrs...))
		// Reset addOpt with new attribute set.
		*addOpt = append((*addOpt)[:0], o)

		e.inst.exportedMetric.Add(e.ctx, e.nMetrics-success, *addOpt...)
	}

	if e.inst.opDuration.Enabled(e.ctx) {
		recOpt := get[metric.RecordOption](recordOptPool)
		defer put(recordOptPool, recOpt)
		*recOpt = append(*recOpt, e.inst.recordOption(err, status))

		d := time.Since(e.start).Seconds()
		e.inst.opDuration.Record(e.ctx, d, *recOpt...)
	}
}

// recordOption returns a RecordOption with attributes representing the
// outcome of the operation being recorded.
//
// If err is nil and status is 200, the default recOpt of the
// Instrumentation is returned.
//
// Otherwise, a new RecordOption is returned with the base attributes of the
// Instrumentation plus the http.response.status_code attribute set to the
// provided status (if non-zero), and if err is not nil, the error.type attribute set
// to the type of the error.
func (i *Instrumentation) recordOption(err error, status int) metric.RecordOption {
	if err == nil && status == http.StatusOK {
		return i.recOpt
	}

	attrs := get[attribute.KeyValue](measureAttrsPool)
	defer put(measureAttrsPool, attrs)
	*attrs = append(*attrs, i.attrs...)

	if status != 0 {
		*attrs = append(*attrs, semconv.HTTPResponseStatusCode(status))
	}
	if err != nil {
		*attrs = append(*attrs, semconv.ErrorType(err))
	}

	// Do not inefficiently make a copy of attrs by using WithAttributes
	// instead of WithAttributeSet.
	return metric.WithAttributeSet(attribute.NewSet(*attrs...))
}

// successful returns the number of successfully exported metrics out of the n
// that were exported based on the provided error.
//
// If err is nil, n is returned. All metrics were successfully exported.
//
// If err is not nil and not an [internal.PartialSuccess] error, 0 is returned.
// It is assumed all metrics failed to be exported.
//
// If err is an [internal.PartialSuccess] error, the number of successfully
// exported metrics is computed by subtracting the RejectedItems field from n. If
// RejectedItems is negative, n is returned. If RejectedItems is greater than
// n, 0 is returned.
func successful(n int64, err error) int64 {
	if err == nil {
		return n // All metrics successfully exported.
	}
	// Split rejection calculation so successful is inlinable.
	return n - rejected(n, err)
}

var errPartialPool = &sync.Pool{
	New: func() any { return new(internal.PartialSuccess) },
}

// rejected returns how many out of the n metrics were rejected based on the
// provided non-nil err.
func rejected(n int64, err error) int64 {
	ps := errPartialPool.Get().(*internal.PartialSuccess)
	defer errPartialPool.Put(ps)
	// Check for partial success.
	if errors.As(err, ps) {
		// Bound RejectedItems to [0, n]. This should not be needed,
		// but be defensive as this is from an external source.
		return min(max(ps.RejectedItems, 0), n)
	}
	return n // All metrics rejected.
}
