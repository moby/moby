package experimental

import (
	"context"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/expctxkeys"
)

// ImportResolver is an experimental func type that, if set,
// will be used as the first step in resolving imports.
// See issue 2294.
// If the import name is not found, it should return nil.
type ImportResolver func(name string) api.Module

// WithImportResolver returns a new context with the given ImportResolver.
func WithImportResolver(ctx context.Context, resolver ImportResolver) context.Context {
	return context.WithValue(ctx, expctxkeys.ImportResolverKey{}, resolver)
}
