//go:build go1.21

package singleflight

import "context"

// withoutCancel returns a copy of parent that is not canceled when parent is canceled.
// The returned context returns no Deadline or Err, and its Done channel is nil.
// Calling [Cause] on the returned context returns nil.
func withoutCancel(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
