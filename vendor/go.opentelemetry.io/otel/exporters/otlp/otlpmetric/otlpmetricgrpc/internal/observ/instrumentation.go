// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package observ provides self-observability metrics for OTLP metric exporters.
// This is an experimental feature controlled by the x.Observability feature flag.
package observ // import "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc/internal/observ"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/semconv/v1.41.0/otelconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc/internal"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc/internal/x"
)

var (
	attrPool = sync.Pool{
		New: func() any {
			// Pre-allocate for common attributes and dynamic error attributes.
			const n = 1 /* otel.component.type */ +
				1 /* otel.component.name */ +
				1 /* server.address */ +
				1 /* server.port */ +
				1 /* error.type */ +
				1 /* rpc.grpc.status_code */
			s := make([]attribute.KeyValue, 0, n)
			return &s
		},
	}

	recOptPool = sync.Pool{
		New: func() any {
			o := make([]metric.RecordOption, 0, 1)
			return &o
		},
	}
)

// Instrumentation holds the self-observability metric instruments for an OTLP metric exporter.
type Instrumentation struct {
	exported otelconv.SDKExporterMetricDataPointExported
	inflight otelconv.SDKExporterMetricDataPointInflight
	duration otelconv.SDKExporterOperationDuration
	attrs    []attribute.KeyValue
	addOpt   metric.AddOption
	recOpt   metric.RecordOption
}

// NewInstrumentation returns instrumentation for an OTLP over gRPC metric
// exporter with the provided ID using the global MeterProvider.
//
// The id should be the unique exporter instance ID. It is used
// to set the "component.name" attribute.
//
// The target is the endpoint the exporter is exporting to.
//
// If the experimental observability is disabled, nil is returned.
func NewInstrumentation(id int64, target string) (*Instrumentation, error) {
	if !x.Observability.Enabled() {
		return nil, nil
	}

	em := &Instrumentation{}

	meter := otel.GetMeterProvider().Meter(
		"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc",
		metric.WithInstrumentationVersion(sdk.Version()),
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	var err error
	var instrumentErr error

	em.exported, instrumentErr = otelconv.NewSDKExporterMetricDataPointExported(meter)
	if instrumentErr != nil {
		err = errors.Join(err, fmt.Errorf("failed to create exported metric: %w", instrumentErr))
	}

	em.inflight, instrumentErr = otelconv.NewSDKExporterMetricDataPointInflight(meter)
	if instrumentErr != nil {
		err = errors.Join(err, fmt.Errorf("failed to create inflight metric: %w", instrumentErr))
	}

	em.duration, instrumentErr = otelconv.NewSDKExporterOperationDuration(meter)
	if instrumentErr != nil {
		err = errors.Join(err, fmt.Errorf("failed to create duration metric: %w", instrumentErr))
	}

	em.attrs = BaseAttrs(id, target)

	attrSet := attribute.NewSet(em.attrs...)
	em.addOpt = metric.WithAttributeSet(attrSet)
	em.recOpt = metric.WithAttributeSet(attribute.NewSet(append(
		[]attribute.KeyValue{semconv.RPCResponseStatusCode(codes.OK.String())},
		em.attrs...,
	)...))

	return em, err
}

// ComponentName returns the component name for the exporter with the
// provided ID.
func ComponentName(id int64) string {
	t := string(otelconv.ComponentTypeOtlpGRPCMetricExporter)
	return fmt.Sprintf("%s/%d", t, id)
}

// BaseAttrs returns the base attributes for the exporter with the provided ID
// and target.
//
// The id should be the unique exporter instance ID. It is used
// to set the "component.name" attribute.
//
// The target is the gRPC target the exporter is exporting to. It is expected
// to be the output of the Client's CanonicalTarget method.
func BaseAttrs(id int64, target string) []attribute.KeyValue {
	host, port, err := ParseCanonicalTarget(target)
	if err != nil || (host == "" && port < 0) {
		if err != nil {
			global.Debug("failed to parse target", "target", target, "error", err)
		}
		return []attribute.KeyValue{
			semconv.OTelComponentName(ComponentName(id)),
			semconv.OTelComponentTypeKey.String(string(otelconv.ComponentTypeOtlpGRPCMetricExporter)),
		}
	}

	// Do not use append so the slice is exactly allocated.

	if port < 0 {
		return []attribute.KeyValue{
			semconv.OTelComponentName(ComponentName(id)),
			semconv.OTelComponentTypeKey.String(string(otelconv.ComponentTypeOtlpGRPCMetricExporter)),
			semconv.ServerAddress(host),
		}
	}

	if host == "" {
		return []attribute.KeyValue{
			semconv.OTelComponentName(ComponentName(id)),
			semconv.OTelComponentTypeKey.String(string(otelconv.ComponentTypeOtlpGRPCMetricExporter)),
			semconv.ServerPort(port),
		}
	}

	return []attribute.KeyValue{
		semconv.OTelComponentName(ComponentName(id)),
		semconv.OTelComponentTypeKey.String(string(otelconv.ComponentTypeOtlpGRPCMetricExporter)),
		semconv.ServerAddress(host),
		semconv.ServerPort(port),
	}
}

// TrackExport tracks an export operation and returns an ExportOp to complete the tracking.
func (em *Instrumentation) TrackExport(ctx context.Context, rm *metricpb.ResourceMetrics) ExportOp {
	if em == nil {
		return ExportOp{}
	}
	start := time.Now()

	var dataPointCount int64
	inflightEnabled := em.inflight.Enabled(ctx)
	exportedEnabled := em.exported.Enabled(ctx)

	if inflightEnabled || exportedEnabled {
		dataPointCount = countProtoDataPoints(rm)
	}

	if inflightEnabled {
		em.inflight.Inst().Add(ctx, dataPointCount, em.addOpt)
	}

	return ExportOp{
		ctx:            ctx,
		start:          start,
		dataPointCount: dataPointCount,
		inst:           em,
	}
}

// ExportOp tracks the operation being observed by [Instrumentation.TrackExport].
type ExportOp struct {
	ctx            context.Context
	start          time.Time
	dataPointCount int64

	inst *Instrumentation
}

// End completes the observation of the operation being observed by a call to
// [Instrumentation.TrackExport].
//
// Any error that is encountered is provided as err.
func (e ExportOp) End(err error) {
	if e.inst == nil {
		return
	}
	if e.inst.inflight.Enabled(e.ctx) {
		e.inst.inflight.Inst().Add(e.ctx, -e.dataPointCount, e.inst.addOpt)
	}

	success := successful(e.dataPointCount, err)
	// Record successfully exported data points, even if the value is 0 which are
	// meaningful to distribution aggregations.
	if e.inst.exported.Enabled(e.ctx) {
		e.inst.exported.Inst().Add(e.ctx, success, e.inst.addOpt)
	}

	if err != nil && e.inst.exported.Enabled(e.ctx) {
		attrsPtr := attrPool.Get().(*[]attribute.KeyValue)
		defer func() {
			*attrsPtr = (*attrsPtr)[:0]
			attrPool.Put(attrsPtr)
		}()

		*attrsPtr = append(*attrsPtr, e.inst.attrs...)
		*attrsPtr = append(*attrsPtr, semconv.ErrorType(err))

		set := attribute.NewSet(*attrsPtr...)
		e.inst.exported.Inst().Add(e.ctx, e.dataPointCount-success, metric.WithAttributeSet(set))
	}

	if e.inst.duration.Enabled(e.ctx) {
		d := time.Since(e.start).Seconds()
		if err != nil {
			recOptPtr := recOptPool.Get().(*[]metric.RecordOption)
			defer func() {
				*recOptPtr = (*recOptPtr)[:0]
				recOptPool.Put(recOptPtr)
			}()

			attrsPtr := attrPool.Get().(*[]attribute.KeyValue)
			defer func() {
				*attrsPtr = (*attrsPtr)[:0]
				attrPool.Put(attrsPtr)
			}()

			*attrsPtr = append(*attrsPtr, e.inst.attrs...)
			*attrsPtr = append(
				*attrsPtr,
				semconv.ErrorType(err),
				semconv.RPCResponseStatusCode(status.Code(err).String()),
			)

			set := attribute.NewSet(*attrsPtr...)
			*recOptPtr = append(*recOptPtr, metric.WithAttributeSet(set))

			e.inst.duration.Inst().Record(e.ctx, d, *recOptPtr...)
		} else {
			e.inst.duration.Inst().Record(e.ctx, d, e.inst.recOpt)
		}
	}
}

// countProtoDataPoints counts the total number of data points in a ResourceMetrics.
func countProtoDataPoints(rm *metricpb.ResourceMetrics) int64 {
	if rm == nil {
		return 0
	}

	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case *metricpb.Metric_Gauge:
				if data.Gauge != nil {
					total += int64(len(data.Gauge.DataPoints))
				}
			case *metricpb.Metric_Sum:
				if data.Sum != nil {
					total += int64(len(data.Sum.DataPoints))
				}
			case *metricpb.Metric_Histogram:
				if data.Histogram != nil {
					total += int64(len(data.Histogram.DataPoints))
				}
			case *metricpb.Metric_ExponentialHistogram:
				if data.ExponentialHistogram != nil {
					total += int64(len(data.ExponentialHistogram.DataPoints))
				}
			case *metricpb.Metric_Summary:
				if data.Summary != nil {
					total += int64(len(data.Summary.DataPoints))
				}
			}
		}
	}
	return total
}

// successful returns the number of successfully exported data points out of the n
// that were exported based on the provided error.
//
// If err is nil, n is returned. All data points were successfully exported.
//
// If err is not nil and not an [internal.PartialSuccess] error, 0 is returned.
// It is assumed all data points failed to be exported.
//
// If err is an [internal.PartialSuccess] error, the number of successfully
// exported data points is computed by subtracting the RejectedItems field from n. If
// RejectedItems is negative, n is returned. If RejectedItems is greater than
// n, 0 is returned.
func successful(n int64, err error) int64 {
	if err == nil {
		return n // All data points successfully exported.
	}
	// Split rejection calculation so successful is inlinable.
	return n - rejected(n, err)
}

var errPartialPool = &sync.Pool{
	New: func() any { return new(internal.PartialSuccess) },
}

// rejected returns how many out of the n data points exporter were rejected based on
// the provided non-nil err.
func rejected(n int64, err error) int64 {
	ps := errPartialPool.Get().(*internal.PartialSuccess)
	defer errPartialPool.Put(ps)
	// Check for partial success.
	if errors.As(err, ps) {
		// Bound RejectedItems to [0, n]. This should not be needed,
		// but be defensive as this is from an external source.
		return min(max(ps.RejectedItems, 0), n)
	}
	return n // All data points rejected.
}
