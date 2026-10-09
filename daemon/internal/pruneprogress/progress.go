// Package pruneprogress provides request-scoped notifications for pruning.
package pruneprogress

import (
	"context"

	"github.com/moby/moby/api/types/jsonstream"
)

type reporterKey struct{}

// WithReporter attaches an optional progress reporter to a prune request.
// The reporter is called synchronously, only after a successful deletion.
func WithReporter(ctx context.Context, report func(jsonstream.Message)) context.Context {
	return context.WithValue(ctx, reporterKey{}, report)
}

// Notify reports a successful deletion, if the request has a reporter.
func Notify(ctx context.Context, id, action string) {
	if report, ok := ctx.Value(reporterKey{}).(func(jsonstream.Message)); ok {
		report(jsonstream.Message{ID: id, Status: action})
	}
}
