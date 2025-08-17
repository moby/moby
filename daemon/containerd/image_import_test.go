package containerd

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/moby/moby/api/types/container"
)

// regression test for https://github.com/moby/moby/issues/45904
func TestContainerConfigToDockerImageConfig(t *testing.T) {
	ociCFG := containerConfigToDockerOCIImageConfig(&container.Config{
		ExposedPorts: container.PortSet{
			"80/tcp": struct{}{},
		},
	})

	expected := map[string]struct{}{"80/tcp": {}}
	assert.Check(t, is.DeepEqual(ociCFG.ExposedPorts, expected))
}
