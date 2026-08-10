package containerimage

import (
	"testing"

	"github.com/moby/buildkit/solver/pb"
	bkcontainerimage "github.com/moby/buildkit/source/containerimage"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

func TestRegistryIdentifierImageChecksum(t *testing.T) {
	checksum := digest.FromString("test")

	id, err := (&Source{}).registryIdentifier("docker.io/library/alpine:latest", map[string]string{
		pb.AttrImageChecksum: checksum.String(),
	}, nil)
	assert.NilError(t, err)

	assert.Equal(t, id.(*bkcontainerimage.ImageIdentifier).Checksum, checksum)
}

func TestRegistryIdentifierInvalidImageChecksum(t *testing.T) {
	_, err := (&Source{}).registryIdentifier("docker.io/library/alpine:latest", map[string]string{
		pb.AttrImageChecksum: "invalid",
	}, nil)
	assert.ErrorContains(t, err, "invalid image checksum")
}
