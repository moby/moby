package presignedurl

import (
	"context"

	"github.com/aws/smithy-go/middleware"
)

// WithIsPresigning adds the isPresigning sentinel value to a context to signal
// that the middleware stack is using the presign flow.
//
// Scoped to stack values. Use github.com/aws/smithy-go/middleware#ClearStackValues
// to clear all stack values.
func WithIsPresigning(ctx context.Context) context.Context {
	return middleware.WithStackValue(ctx, isPresigningKey{}, true)
}

// GetIsPresigning returns if the context contains the isPresigning sentinel
// value for presigning flows.
//
// Scoped to stack values. Use github.com/aws/smithy-go/middleware#ClearStackValues
// to clear all stack values.
func GetIsPresigning(ctx context.Context) bool {
	v, _ := middleware.GetStackValue(ctx, isPresigningKey{}).(bool)
	return v
}

type isPresigningKey struct{}

// AddAsIsPresigingMiddleware adds a middleware to the head of the stack that
// will update the stack's context to be flagged as being invoked for the
// purpose of presigning.
func AddAsIsPresigingMiddleware(stack *middleware.Stack) error {
	return stack.Initialize.Add(asIsPresigningMiddleware{}, middleware.Before)
}

type asIsPresigningMiddleware struct{}

func (asIsPresigningMiddleware) ID() string { return "AsIsPresigningMiddleware" }

func (asIsPresigningMiddleware) HandleInitialize(
	ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler,
) (
	out middleware.InitializeOutput, metadata middleware.Metadata, err error,
) {
	ctx = WithIsPresigning(ctx)
	return next.HandleInitialize(ctx, in)
}
