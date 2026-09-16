package containernamegeneratorv0

import (
	"context"
	"testing"

	"github.com/moby/extensions"
	"github.com/moby/moby/v2/internal/testutil/staticresolver"
	"gotest.tools/v3/assert"
)

type generatorFunc func(context.Context, *GenerateContainerNameRequest) (*GenerateContainerNameReply, error)

func (f generatorFunc) GenerateContainerName(ctx context.Context, req *GenerateContainerNameRequest) (*GenerateContainerNameReply, error) {
	return f(ctx, req)
}

func TestGenerateContainerName(t *testing.T) {
	t.Parallel()

	provider := generatorFunc(func(_ context.Context, req *GenerateContainerNameRequest) (*GenerateContainerNameReply, error) {
		assert.Equal(t, req.Retry, int64(3))
		assert.Equal(t, req.ContainerID, "container-id")
		assert.Equal(t, req.Image, "image:latest")
		return &GenerateContainerNameReply{Name: "container-name"}, nil
	})
	resolver := staticresolver.New(extensions.ResolvedProvider{
		Identity: extensions.ExtensionIdentity{
			ID:     "org.example.containernamegenerator.v1",
			Origin: extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginBuiltin},
		},
		Impl: provider,
	})

	reply, err := GenerateContainerName(context.Background(), resolver, &GenerateContainerNameRequest{
		Retry:       3,
		ContainerID: "container-id",
		Image:       "image:latest",
	})
	assert.NilError(t, err)
	assert.Equal(t, reply.Name, "container-name")
}
