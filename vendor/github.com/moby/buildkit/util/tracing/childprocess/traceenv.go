package childprocess

import (
	"context"
	"os"

	"github.com/moby/buildkit/util/appcontext"
	"go.opentelemetry.io/otel/propagation"
)

func init() {
	appcontext.Register(initContext)
}

func initContext(ctx context.Context) context.Context {
	// open-telemetry/opentelemetry-specification#740
	parent := os.Getenv("TRACEPARENT")
	state := os.Getenv("TRACESTATE")

	if parent != "" {
		tc := propagation.TraceContext{}
		return tc.Extract(ctx, &textMap{parent: parent, state: state})
	}

	// deprecated: removed in v0.11.0
	// previously defined in https://github.com/open-telemetry/opentelemetry-swift/blob/4ea467ed4b881d7329bf2254ca7ed7f2d9d6e1eb/Sources/OpenTelemetrySdk/Trace/Propagation/EnvironmentContextPropagator.swift#L14-L15
	parent = os.Getenv("OTEL_TRACE_PARENT")
	state = os.Getenv("OTEL_TRACE_STATE")

	if parent == "" {
		return ctx
	}

	tc := propagation.TraceContext{}
	return tc.Extract(ctx, &textMap{parent: parent, state: state})
}
