package detect

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

const (
	traceparentHeader = "traceparent"
	tracestateHeader  = "tracestate"
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

type textMap struct {
	parent string
	state  string
}

func (tm *textMap) Get(key string) string {
	switch key {
	case traceparentHeader:
		return tm.parent
	case tracestateHeader:
		return tm.state
	default:
		return ""
	}
}

func (tm *textMap) Set(key string, value string) {
	switch key {
	case traceparentHeader:
		tm.parent = value
	case tracestateHeader:
		tm.state = value
	}
}

func (tm *textMap) Keys() []string {
	var k []string
	if tm.parent != "" {
		k = append(k, traceparentHeader)
	}
	if tm.state != "" {
		k = append(k, tracestateHeader)
	}
	return k
}
