package servicenamegeneratorv0

import (
	"context"
	"testing"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/internal/testutil/staticresolver"
	"gotest.tools/v3/assert"
)

type generatorFunc func(context.Context, *GenerateServiceNameRequest) (*GenerateServiceNameReply, error)

func (f generatorFunc) GenerateServiceName(ctx context.Context, req *GenerateServiceNameRequest) (*GenerateServiceNameReply, error) {
	return f(ctx, req)
}

func TestGenerateServiceName(t *testing.T) {
	t.Parallel()

	provider := generatorFunc(func(_ context.Context, req *GenerateServiceNameRequest) (*GenerateServiceNameReply, error) {
		assert.Equal(t, req.Retry, int64(3))
		assert.Equal(t, req.Image, "image:latest")
		return &GenerateServiceNameReply{Name: "service-name"}, nil
	})
	resolver := staticresolver.New(extensions.ResolvedProvider{
		Identity: extensions.ExtensionIdentity{
			ID:     "org.example.servicenamegenerator.v1",
			Origin: extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginBuiltin},
		},
		Impl: provider,
	})

	reply, err := GenerateServiceName(context.Background(), resolver, &GenerateServiceNameRequest{
		Retry: 3,
		Image: "image:latest",
	})
	assert.NilError(t, err)
	assert.Equal(t, reply.Name, "service-name")
}
