package daemon

import (
	"testing"

	networkSettings "github.com/moby/moby/v2/daemon/network"
	containerruntimev0 "github.com/moby/moby/v2/extpoints/container/runtime/v0"
	"github.com/moby/moby/v2/pkg/sysinfo"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBackendCreateConfigResolvesDefaultNetwork(t *testing.T) {
	t.Parallel()

	cfg, err := backendCreateConfig(&containerruntimev0.CreateFromAPIRequest{
		Name:             "job",
		Config:           []byte(`{"Image": "busybox"}`),
		HostConfig:       []byte(`{"NetworkMode": "default"}`),
		NetworkingConfig: []byte(`{"EndpointsConfig": {"default": {"Aliases": ["job"]}}}`),
	}, &sysinfo.SysInfo{})
	assert.NilError(t, err)

	mode := cfg.HostConfig.NetworkMode
	assert.Check(t, is.Equal(cfg.Name, "job"))
	assert.Check(t, is.Equal(cfg.Config.Image, "busybox"))
	assert.Check(t, is.Equal(string(mode), networkSettings.DefaultNetwork))
	assert.Assert(t, is.Len(cfg.NetworkingConfig.EndpointsConfig, 1))
	assert.Check(t, is.DeepEqual(cfg.NetworkingConfig.EndpointsConfig[mode.NetworkName()].Aliases, []string{"job"}))
}
