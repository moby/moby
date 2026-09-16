package extensions

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/log"
)

// Policy controls provider deadlines and error handling for [Each] and [Fold].
type Policy struct {
	// Timeout bounds each provider call independently. Zero leaves the calls
	// bounded only by the caller's context.
	//
	// It is enforced for out-of-process providers, which receive the deadline
	// over gRPC. An in-process provider is passed the same context but only
	// trusted to honor it: abandoning a direct Go call would leave the provider
	// mutating shared engine state after the caller moved on. State a
	// fail-closed point's guarantee accordingly.
	Timeout time.Duration

	// FailOpen continues with the remaining providers when one returns an error.
	// Veto points should leave it false.
	FailOpen bool

	// Action names what the provider's error did to the operation, for the
	// wrapped error: `<point> provider "<id>" <action>: <err>`. Leave it empty
	// for a plain failure.
	Action string
}

// wrap attributes err to the extension that produced it.
func (p Policy) wrap(point PointID, extension ExtensionID, err error) error {
	if p.Action == "" {
		return fmt.Errorf("%s provider %q: %w", point.Name(), extension, err)
	}
	return fmt.Errorf("%s provider %q %s: %w", point.Name(), extension, p.Action, err)
}

// Each calls fn once per provider of p under policy. Use it for validation,
// veto, or notification points.
//
// Fan-out order is unspecified. A point that needs a defined order must define
// it (see the guidance in the extensions authoring guide).
func Each[T any](ctx context.Context, p Point[T], r Resolver, policy Policy, fn func(context.Context, T) error) error {
	_, err := Fold(ctx, p, r, policy, struct{}{}, func(ctx context.Context, impl T, _ struct{}) (struct{}, error) {
		return struct{}{}, fn(ctx, impl)
	})
	return err
}

// Fold threads acc through every provider of p and returns the final value under
// policy. Each provider sees the value produced by its predecessors.
//
// A provider that fails under a fail-closed policy aborts the fold and its
// partial value is discarded; under a fail-open one its contribution is dropped
// and the fold continues from the value it was given.
func Fold[T, A any](ctx context.Context, p Point[T], r Resolver, policy Policy, acc A, fn func(context.Context, T, A) (A, error)) (A, error) {
	providers, err := p.All(r)
	if err != nil {
		return acc, err
	}
	for _, provider := range providers {
		next, err := callProvider(ctx, p.id, policy, provider, acc, fn)
		if err != nil {
			if !policy.FailOpen {
				// Discard the partial value rather than hand back a half-applied
				// one: for a composing point such as the create-spec hook that
				// would be an OCI spec carrying some providers' changes but not
				// the rest, which is worse than no answer.
				var zero A
				return zero, err
			}
			log.G(ctx).WithError(err).Warn("extensions: skipping failed provider")
			continue
		}
		acc = next
	}
	return acc, nil
}

// callProvider gives each provider its own timeout and releases it when the
// call returns.
func callProvider[T, A any](ctx context.Context, point PointID, policy Policy, provider TypedProvider[T], acc A, fn func(context.Context, T, A) (A, error)) (A, error) {
	if policy.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, policy.Timeout)
		defer cancel()
	}
	next, err := fn(ctx, provider.Impl, acc)
	if err != nil {
		return acc, policy.wrap(point, provider.Identity.ID, err)
	}
	return next, nil
}
