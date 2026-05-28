package childprocess

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

// Environ returns list of environment variables that need to be sent to the child process
// in order for it to pick up cross-process tracing from same state.
func Environ(ctx context.Context) []string {
	var tm textMap
	tc := propagation.TraceContext{}
	tc.Inject(ctx, &tm)

	var env []string

	// deprecated: removed in v0.11.0
	// previously defined in https://github.com/open-telemetry/opentelemetry-swift/blob/4ea467ed4b881d7329bf2254ca7ed7f2d9d6e1eb/Sources/OpenTelemetrySdk/Trace/Propagation/EnvironmentContextPropagator.swift#L14-L15
	if tm.parent != "" {
		env = append(env, "OTEL_TRACE_PARENT="+tm.parent)
	}
	if tm.state != "" {
		env = append(env, "OTEL_TRACE_STATE="+tm.state)
	}

	// open-telemetry/opentelemetry-specification#740
	if tm.parent != "" {
		env = append(env, "TRACEPARENT="+tm.parent)
	}
	if tm.state != "" {
		env = append(env, "TRACESTATE="+tm.state)
	}

	return env
}
