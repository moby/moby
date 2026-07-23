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
	parent := os.Getenv("TRACEPARENT") // https://www.w3.org/TR/trace-context/#traceparent-header
	state := os.Getenv("TRACESTATE")   // https://www.w3.org/TR/trace-context/#tracestate-header

	if parent != "" {
		tc := propagation.TraceContext{}
		return tc.Extract(ctx, &textMap{parent: parent, state: state})
	}

	return ctx
}
