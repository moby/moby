package containerd

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// regression test for https://github.com/moby/moby/issues/45904
func TestContainerConfigToDockerImageConfig(t *testing.T) {
	ociCFG := containerConfigToDockerOCIImageConfig(&container.Config{
		ExposedPorts: map[container.PortRangeProto]struct{}{
			"80/tcp": {},
		},
	})

	expected := map[string]struct{}{"80/tcp": {}}
	assert.Check(t, is.DeepEqual(ociCFG.ExposedPorts, expected))
}
