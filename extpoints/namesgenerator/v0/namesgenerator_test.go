package namesgeneratorv0

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/internal/testutil/staticresolver"
	"gotest.tools/v3/assert"
)

type generatorFunc func(context.Context, *GenerateRequest) (*GenerateReply, error)

func (f generatorFunc) Generate(ctx context.Context, req *GenerateRequest) (*GenerateReply, error) {
	return f(ctx, req)
}

func TestGenerateUsesReplacementProvider(t *testing.T) {
	t.Parallel()

	builtinCalled := false
	provider := generatorFunc(func(ctx context.Context, req *GenerateRequest) (*GenerateReply, error) {
		deadline, ok := ctx.Deadline()
		assert.Assert(t, ok)
		assert.Assert(t, time.Until(deadline) <= 5*time.Second)
		assert.Equal(t, req.Retry, int64(3))
		return &GenerateReply{Name: "replacement-name"}, nil
	})
	builtin := generatorFunc(func(context.Context, *GenerateRequest) (*GenerateReply, error) {
		builtinCalled = true
		return &GenerateReply{Name: "builtin-name"}, nil
	})

	resolver := staticresolver.New(
		extensions.ResolvedProvider{Extension: "org.example.replacement.v1", Impl: provider},
		extensions.ResolvedProvider{Extension: "org.example.builtin.v1", Impl: builtin, Builtin: true},
	)
	reply, err := Generate(context.Background(), resolver, &GenerateRequest{Retry: 3})
	assert.NilError(t, err)
	assert.Equal(t, reply.Name, "replacement-name")
	assert.Assert(t, !builtinCalled)
}

func TestGenerateFallsBackToBuiltinForInvalidReplacementReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		generate generatorFunc
		wantErr  string
	}{
		{
			name: "error",
			generate: func(context.Context, *GenerateRequest) (*GenerateReply, error) {
				return nil, errors.New("replacement failed")
			},
			wantErr: "replacement failed",
		},
		{
			name: "nil reply",
			generate: func(context.Context, *GenerateRequest) (*GenerateReply, error) {
				return nil, nil
			},
			wantErr: "provider returned a nil reply",
		},
		{
			name: "empty name",
			generate: func(context.Context, *GenerateRequest) (*GenerateReply, error) {
				return &GenerateReply{}, nil
			},
			wantErr: "provider returned an empty name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			builtinCalls := 0
			builtin := generatorFunc(func(ctx context.Context, _ *GenerateRequest) (*GenerateReply, error) {
				_, ok := ctx.Deadline()
				assert.Assert(t, ok, "the fallback attempt must have its own deadline")
				builtinCalls++
				return &GenerateReply{Name: "builtin-name"}, nil
			})
			resolver := staticresolver.New(
				extensions.ResolvedProvider{Extension: "org.example.replacement.v1", Impl: tc.generate},
				extensions.ResolvedProvider{Extension: "org.example.builtin.v1", Impl: builtin, Builtin: true},
			)

			reply, err := Generate(context.Background(), resolver, &GenerateRequest{})
			assert.ErrorContains(t, err, `namesgenerator provider "org.example.replacement.v1"`)
			assert.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, reply.Name, "builtin-name")
			assert.Equal(t, builtinCalls, 1)
		})
	}
}

func TestGenerateDoesNotRetryFailingBuiltin(t *testing.T) {
	t.Parallel()

	calls := 0
	builtin := generatorFunc(func(context.Context, *GenerateRequest) (*GenerateReply, error) {
		calls++
		return nil, errors.New("builtin failed")
	})

	reply, err := Generate(context.Background(), staticresolver.New(
		extensions.ResolvedProvider{Extension: "org.example.builtin.v1", Impl: builtin, Builtin: true},
	), &GenerateRequest{})
	assert.Assert(t, reply == nil)
	assert.ErrorContains(t, err, `namesgenerator provider "org.example.builtin.v1": builtin failed`)
	assert.Equal(t, calls, 1)
}

func TestGenerateJoinsReplacementAndBuiltinErrors(t *testing.T) {
	t.Parallel()

	replacementErr := errors.New("replacement failed")
	builtinErr := errors.New("builtin failed")
	resolver := staticresolver.New(
		extensions.ResolvedProvider{Extension: "org.example.replacement.v1", Impl: generatorFunc(func(context.Context, *GenerateRequest) (*GenerateReply, error) {
			return nil, replacementErr
		})},
		extensions.ResolvedProvider{Extension: "org.example.builtin.v1", Impl: generatorFunc(func(context.Context, *GenerateRequest) (*GenerateReply, error) {
			return nil, builtinErr
		}), Builtin: true},
	)

	reply, err := Generate(context.Background(), resolver, &GenerateRequest{})
	assert.Assert(t, reply == nil)
	assert.Assert(t, errors.Is(err, replacementErr))
	assert.Assert(t, errors.Is(err, builtinErr))
	assert.ErrorContains(t, err, `namesgenerator provider "org.example.replacement.v1"`)
	assert.ErrorContains(t, err, `namesgenerator provider "org.example.builtin.v1"`)
}

func TestGenerateRequiresExactlyOneEffectiveProvider(t *testing.T) {
	t.Parallel()

	provider := generatorFunc(func(context.Context, *GenerateRequest) (*GenerateReply, error) {
		return &GenerateReply{Name: "name"}, nil
	})

	_, err := Generate(context.Background(), staticresolver.New(), &GenerateRequest{})
	assert.ErrorContains(t, err, "has no providers")

	_, err = Generate(context.Background(), staticresolver.New(
		extensions.ResolvedProvider{Extension: "org.example.one.v1", Impl: provider},
		extensions.ResolvedProvider{Extension: "org.example.two.v1", Impl: provider},
	), &GenerateRequest{})
	assert.ErrorContains(t, err, "has multiple providers")
}

func TestGeneratePropagatesContextTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	provider := generatorFunc(func(ctx context.Context, _ *GenerateRequest) (*GenerateReply, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	_, err := Generate(ctx, staticresolver.New(
		extensions.ResolvedProvider{Extension: "org.example.slow.v1", Impl: provider},
	), &GenerateRequest{})
	assert.Assert(t, errors.Is(err, context.DeadlineExceeded))
	assert.ErrorContains(t, err, `namesgenerator provider "org.example.slow.v1"`)
}
