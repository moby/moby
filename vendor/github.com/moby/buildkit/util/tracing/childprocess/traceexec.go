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

	// open-telemetry/opentelemetry-specification#740
	if tm.parent != "" {
		env = append(env, "TRACEPARENT="+tm.parent)
	}
	if tm.state != "" {
		env = append(env, "TRACESTATE="+tm.state)
	}

	return env
}
