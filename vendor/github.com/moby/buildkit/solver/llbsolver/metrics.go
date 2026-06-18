package llbsolver

import (
	"context"

	controlapi "github.com/moby/buildkit/api/services/control"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"google.golang.org/grpc/codes"
)

// instrumentationName is the OTEL instrumentation scope used for every
// build observability instrument exported by the solver.
const instrumentationName = "github.com/moby/buildkit/solver/llbsolver"

// Attribute keys for build observability metrics.
const (
	statusKey    = attribute.Key("status")
	errorCodeKey = attribute.Key("error_code")
	kindKey      = attribute.Key("kind")
)

// Attribute values for the keys above.
const (
	statusSuccess = "success"
	statusFailure = "failure"

	stepKindCompleted = "completed"
	stepKindCached    = "cached"
	stepKindTotal     = "total"
	stepKindWarnings  = "warnings"
)

// buildMetrics holds the OTEL instruments the solver writes to once per
// solve completion. The struct is constructed once in New and shared by
// every Solve via the *Solver receiver — instruments are concurrency-safe
// per the OTEL metric API contract.
type buildMetrics struct {
	builds   metric.Int64Counter
	steps    metric.Int64Counter
	duration metric.Float64Histogram
}

// newBuildMetrics registers the build-completion instruments against mp.
// A nil mp is treated as a request to disable metrics: a no-op provider
// is used so the rest of the solver does not need to special-case the
// "metrics disabled" path. Returning an error here causes daemon startup
// to fail fast, matching the buildkit convention for required wiring.
func newBuildMetrics(mp metric.MeterProvider) (*buildMetrics, error) {
	if mp == nil {
		mp = noop.NewMeterProvider()
	}
	meter := mp.Meter(instrumentationName)

	builds, err := meter.Int64Counter(
		"buildkit.builds",
		metric.WithDescription("Number of builds completed, labeled by frontend, status, and (on failure) gRPC error code."),
	)
	if err != nil {
		return nil, err
	}

	steps, err := meter.Int64Counter(
		"buildkit.builds.steps",
		metric.WithDescription("Cumulative count of build steps observed, partitioned by kind (completed, cached, total, warnings)."),
	)
	if err != nil {
		return nil, err
	}

	duration, err := meter.Float64Histogram(
		"buildkit.build.duration",
		metric.WithDescription("Wall-clock duration of build solves from CreatedAt to CompletedAt."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &buildMetrics{
		builds:   builds,
		steps:    steps,
		duration: duration,
	}, nil
}

// recordBuildCompletion observes one finished build onto every relevant
// instrument. It is invoked from the recordBuildHistory finalizer, after
// rec is fully populated and immediately before the COMPLETE history
// event is sent.
//
// Labels are deliberately bounded to keep cardinality finite:
//   - status: "success" or "failure".
//   - error_code: the gRPC codes.Code string of rec.Error, attached only
//     on failure. rec.Error.Message is intentionally not used as a label
//     to avoid the cardinality blow-up reported in moby/buildkit#5777.
//
// rec.Frontend is intentionally NOT used as a label: the modern
// gateway-client path used by buildctl and buildx clears Frontend on
// the wire (see client/build.go), so the field is empty for nearly all
// real-world callers and the label would have cardinality 1. A
// follow-up can re-introduce a frontend signal if and when buildkit
// itself starts populating rec.Frontend on the gateway path.
//
// A nil receiver or nil rec is a no-op so callers do not have to guard
// the call site.
func (m *buildMetrics) recordBuildCompletion(ctx context.Context, rec *controlapi.BuildHistoryRecord) {
	if m == nil || rec == nil {
		return
	}

	status := statusSuccess
	attrs := []attribute.KeyValue{statusKey.String(status)}
	if rec.Error != nil {
		status = statusFailure
		attrs[0] = statusKey.String(status)
		attrs = append(attrs, errorCodeKey.String(codes.Code(rec.Error.Code).String()))
	}

	m.builds.Add(ctx, 1, metric.WithAttributes(attrs...))

	m.steps.Add(ctx, int64(rec.NumCompletedSteps), metric.WithAttributes(kindKey.String(stepKindCompleted)))
	m.steps.Add(ctx, int64(rec.NumCachedSteps), metric.WithAttributes(kindKey.String(stepKindCached)))
	m.steps.Add(ctx, int64(rec.NumTotalSteps), metric.WithAttributes(kindKey.String(stepKindTotal)))
	m.steps.Add(ctx, int64(rec.NumWarnings), metric.WithAttributes(kindKey.String(stepKindWarnings)))

	if rec.CreatedAt != nil && rec.CompletedAt != nil {
		seconds := rec.CompletedAt.AsTime().Sub(rec.CreatedAt.AsTime()).Seconds()
		m.duration.Record(ctx, seconds, metric.WithAttributes(statusKey.String(status)))
	}
}
