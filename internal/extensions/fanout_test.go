package extensions

import (
	"context"
	"errors"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestEachBoundsEveryProviderIndependently verifies that each provider receives
// a fresh timeout budget.
func TestEachBoundsEveryProviderIndependently(t *testing.T) {
	const timeout = 200 * time.Millisecond
	var budgets []time.Duration
	record := callerFunc(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		assert.Assert(t, ok, "provider must receive a deadline")
		budgets = append(budgets, time.Until(deadline))
		return nil
	})
	slow := callerFunc(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		assert.Assert(t, ok, "provider must receive a deadline")
		budgets = append(budgets, time.Until(deadline))
		time.Sleep(timeout / 2)
		return nil
	})

	err := Each(context.Background(), testPoint, resolverOf(
		ResolvedProvider{Extension: "slow", Impl: slow},
		ResolvedProvider{Extension: "second", Impl: record},
	), Policy{Timeout: timeout}, func(ctx context.Context, c caller) error {
		return c.Call(ctx)
	})
	assert.NilError(t, err)

	assert.Assert(t, is.Len(budgets, 2))
	assert.Assert(t, budgets[1] > timeout/2,
		"second provider got %s of a %s budget; the deadline is shared, not per-provider", budgets[1], timeout)
}

func TestEachAbortsOnErrorAndAttributes(t *testing.T) {
	called := 0
	count := callerFunc(func(context.Context) error { called++; return nil })
	veto := callerFunc(func(context.Context) error { called++; return errors.New("not allowed") })

	err := Each(context.Background(), testPoint, resolverOf(
		ResolvedProvider{Extension: "org.example.veto.v1", Impl: veto},
		ResolvedProvider{Extension: "org.example.after.v1", Impl: count},
	), Policy{Action: "vetoed the start"}, func(ctx context.Context, c caller) error {
		return c.Call(ctx)
	})

	assert.ErrorContains(t, err, `test provider "org.example.veto.v1" vetoed the start: not allowed`)
	assert.Equal(t, called, 1, "a fail-closed point must not call providers after a veto")
}

func TestEachFailOpenSkipsAndContinues(t *testing.T) {
	called := 0
	count := callerFunc(func(context.Context) error { called++; return nil })
	boom := callerFunc(func(context.Context) error { called++; return errors.New("boom") })

	err := Each(context.Background(), testPoint, resolverOf(
		ResolvedProvider{Extension: "org.example.broken.v1", Impl: boom},
		ResolvedProvider{Extension: "org.example.ok.v1", Impl: count},
	), Policy{FailOpen: true}, func(ctx context.Context, c caller) error {
		return c.Call(ctx)
	})

	assert.NilError(t, err)
	assert.Equal(t, called, 2, "a fail-open point must continue past a failing provider")
}

func TestFoldThreadsValueInOrder(t *testing.T) {
	noop := callerFunc(func(context.Context) error { return nil })
	out, err := Fold(context.Background(), testPoint, resolverOf(
		ResolvedProvider{Extension: "a", Impl: noop},
		ResolvedProvider{Extension: "b", Impl: noop},
	), Policy{}, "seed", func(_ context.Context, _ caller, acc string) (string, error) {
		return acc + "+", nil
	})
	assert.NilError(t, err)
	assert.Equal(t, out, "seed++")
}

func TestFoldDiscardsPartialValueOnError(t *testing.T) {
	out, err := Fold(context.Background(), testPoint, resolverOf(
		ResolvedProvider{Extension: "a", Impl: callerFunc(func(context.Context) error { return nil })},
		ResolvedProvider{Extension: "b", Impl: callerFunc(func(context.Context) error { return errors.New("no") })},
	), Policy{}, "seed", func(_ context.Context, c caller, acc string) (string, error) {
		if err := c.Call(context.Background()); err != nil {
			return acc, err
		}
		return acc + "+", nil
	})
	assert.ErrorContains(t, err, "no")
	assert.Equal(t, out, "", "a fail-closed fold must discard the partial value")
}

func TestPointIDName(t *testing.T) {
	for _, tc := range []struct {
		id   PointID
		want string
	}{
		{"org.mobyproject.extension.container.create_spec.v0", "create_spec"},
		{"org.mobyproject.extension.example.greeter.v0", "greeter"},
		{"moby.extensions.internal.launcher.echo.v1", "echo"},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			assert.Equal(t, tc.id.Name(), tc.want)
		})
	}
}

func TestEnabled(t *testing.T) {
	assert.Assert(t, !testPoint.Enabled(resolverOf()))
	assert.Assert(t, testPoint.Enabled(resolverOf(ResolvedProvider{Extension: "a", Impl: callerFunc(nil)})))
}
