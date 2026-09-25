package namesgenerator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/internal/testutil/staticresolver"
	"gotest.tools/v3/assert"
)

var testPoint = extensions.DefineSinglePoint[any]("org.example.namesgenerator.v0")

func resolvedProvider(id extensions.ExtensionID, impl any, builtin bool) extensions.ResolvedProvider {
	origin := extensions.ExtensionOrigin{
		Kind:       extensions.ExtensionOriginExecutable,
		Executable: &extensions.ExecutableOrigin{Path: string(id)},
	}
	if builtin {
		origin = extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginBuiltin}
	}
	return extensions.ResolvedProvider{
		Identity: extensions.ExtensionIdentity{ID: id, Origin: origin},
		Impl:     impl,
	}
}

func TestGenerateUsesReplacementProvider(t *testing.T) {
	t.Parallel()

	builtinCalled := false
	resolver := staticresolver.New(
		resolvedProvider("org.example.replacement.v1", "replacement", false),
		resolvedProvider("org.example.builtin.v1", "builtin", true),
	)
	name, err := Generate(context.Background(), resolver, testPoint.ID(), func(ctx context.Context, impl any) (string, error) {
		deadline, ok := ctx.Deadline()
		assert.Assert(t, ok)
		assert.Assert(t, time.Until(deadline) <= 5*time.Second)
		if impl == "builtin" {
			builtinCalled = true
			return "builtin-name", nil
		}
		return "replacement-name", nil
	})
	assert.NilError(t, err)
	assert.Equal(t, name, "replacement-name")
	assert.Assert(t, !builtinCalled)
}

func TestGenerateFallsBackToBuiltin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		replacementName string
		replacementErr  error
		wantErr         string
	}{
		{name: "error", replacementErr: errors.New("replacement failed"), wantErr: "replacement failed"},
		{name: "empty name", wantErr: "provider returned an empty name"},
		{name: "invalid character", replacementName: "invalid.name", wantErr: "provider returned an invalid name"},
		{name: "too short", replacementName: "x", wantErr: "provider returned an invalid name"},
		{name: "too long", replacementName: strings.Repeat("x", 64), wantErr: "provider returned an invalid name"},
		{name: "ending with separator", replacementName: "invalid_name_", wantErr: "provider returned an invalid name"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			builtinCalls := 0
			resolver := staticresolver.New(
				resolvedProvider("org.example.replacement.v1", "replacement", false),
				resolvedProvider("org.example.builtin.v1", "builtin", true),
			)
			name, err := Generate(context.Background(), resolver, testPoint.ID(), func(ctx context.Context, impl any) (string, error) {
				if impl == "builtin" {
					_, ok := ctx.Deadline()
					assert.Assert(t, ok, "the fallback attempt must have its own deadline")
					builtinCalls++
					return "builtin-name", nil
				}
				return tc.replacementName, tc.replacementErr
			})
			assert.ErrorContains(t, err, `namesgenerator provider "org.example.replacement.v1"`)
			assert.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, name, "builtin-name")
			assert.Equal(t, builtinCalls, 1)
		})
	}
}

func TestGenerateDoesNotRetryFailingBuiltin(t *testing.T) {
	t.Parallel()

	calls := 0
	name, err := Generate(context.Background(), staticresolver.New(
		resolvedProvider("org.example.builtin.v1", "builtin", true),
	), testPoint.ID(), func(context.Context, any) (string, error) {
		calls++
		return "", errors.New("builtin failed")
	})
	assert.Equal(t, name, "")
	assert.ErrorContains(t, err, `namesgenerator provider "org.example.builtin.v1": builtin failed`)
	assert.Equal(t, calls, 1)
}

func TestGenerateJoinsReplacementAndBuiltinErrors(t *testing.T) {
	t.Parallel()

	replacementErr := errors.New("replacement failed")
	builtinErr := errors.New("builtin failed")
	name, err := Generate(context.Background(), staticresolver.New(
		resolvedProvider("org.example.replacement.v1", replacementErr, false),
		resolvedProvider("org.example.builtin.v1", builtinErr, true),
	), testPoint.ID(), func(_ context.Context, impl any) (string, error) {
		return "", impl.(error)
	})
	assert.Equal(t, name, "")
	assert.Assert(t, errors.Is(err, replacementErr))
	assert.Assert(t, errors.Is(err, builtinErr))
}

func TestGenerateRequiresExactlyOneEffectiveProvider(t *testing.T) {
	t.Parallel()

	invoke := func(context.Context, any) (string, error) { return "name", nil }
	_, err := Generate(context.Background(), staticresolver.New(), testPoint.ID(), invoke)
	assert.ErrorContains(t, err, "has no providers")

	_, err = Generate(context.Background(), staticresolver.New(
		resolvedProvider("org.example.one.v1", "one", false),
		resolvedProvider("org.example.two.v1", "two", false),
	), testPoint.ID(), invoke)
	assert.ErrorContains(t, err, "has multiple providers")
}

func TestGeneratePropagatesContextTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := Generate(ctx, staticresolver.New(
		resolvedProvider("org.example.slow.v1", "slow", false),
	), testPoint.ID(), func(ctx context.Context, _ any) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	assert.Assert(t, errors.Is(err, context.DeadlineExceeded))
}
