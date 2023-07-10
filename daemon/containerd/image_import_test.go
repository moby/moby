package containerd

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// regression test for https://github.com/moby/moby/issues/45904
func TestContainerConfigToOciImageConfig(t *testing.T) {
	ociCFG := containerConfigToOciImageConfig(&container.Config{
		ExposedPorts: nat.PortSet{
			"80/tcp": struct{}{},
		},
	})

	expected := map[string]struct{}{"80/tcp": {}}
	assert.Check(t, is.DeepEqual(ociCFG.ExposedPorts, expected))
}
