package experimental

import (
	"context"

	"github.com/tetratelabs/wazero/internal/expctxkeys"
)

// WithCompilationWorkers sets the desired number of compilation workers.
func WithCompilationWorkers(ctx context.Context, workers int) context.Context {
	return context.WithValue(ctx, expctxkeys.CompilationWorkers{}, workers)
}

// GetCompilationWorkers returns the desired number of compilation workers.
// The minimum value returned is 1.
func GetCompilationWorkers(ctx context.Context) int {
	workers, _ := ctx.Value(expctxkeys.CompilationWorkers{}).(int)
	return max(workers, 1)
}
