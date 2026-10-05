package daemon

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	networkSettings "github.com/moby/moby/v2/daemon/network"
	runtimev0 "github.com/moby/moby/v2/extpoints/runtime/v0"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestRuntimeExtensionDeclaration(t *testing.T) {
	// Declaration is nil-daemon safe: the daemon is only dereferenced when
	// the provider serves requests.
	decl := runtimeExtension(nil).Declaration()
	assert.Check(t, is.Equal(string(decl.ID), runtimeExtensionID))
	assert.Assert(t, is.Len(decl.Providers, 1))
	assert.Check(t, is.Equal(decl.Providers[0].Point, runtimev0.Point.ID()))
}

func TestBackendCreateConfig(t *testing.T) {
	t.Run("nil host config resolves to the default network", func(t *testing.T) {
		cfg := backendCreateConfig(runtimev0.ContainerCreateRequest{Name: "job"})
		assert.Check(t, is.Equal(cfg.Name, "job"))
		assert.Assert(t, cfg.HostConfig != nil)
		assert.Check(t, is.Equal(string(cfg.HostConfig.NetworkMode), networkSettings.DefaultNetwork))
	})

	t.Run("default network mode is resolved and its endpoint follows", func(t *testing.T) {
		endpoint := &network.EndpointSettings{}
		cfg := backendCreateConfig(runtimev0.ContainerCreateRequest{
			HostConfig: &container.HostConfig{NetworkMode: network.NetworkDefault},
			NetworkingConfig: &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					network.NetworkDefault: endpoint,
				},
			},
		})
		mode := cfg.HostConfig.NetworkMode
		assert.Check(t, is.Equal(string(mode), networkSettings.DefaultNetwork))
		assert.Check(t, is.Equal(cfg.NetworkingConfig.EndpointsConfig[mode.NetworkName()], endpoint))
		_, stale := cfg.NetworkingConfig.EndpointsConfig[network.NetworkDefault]
		assert.Check(t, !stale, "the default-keyed endpoint must be re-keyed, not duplicated")
	})

	t.Run("empty network mode resolves like default, without mutating the request", func(t *testing.T) {
		endpoint := &network.EndpointSettings{}
		hostConfig := &container.HostConfig{}
		networking := &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				network.NetworkDefault: endpoint,
			},
		}
		cfg := backendCreateConfig(runtimev0.ContainerCreateRequest{
			HostConfig:       hostConfig,
			NetworkingConfig: networking,
		})
		mode := cfg.HostConfig.NetworkMode
		assert.Check(t, is.Equal(string(mode), networkSettings.DefaultNetwork))
		assert.Check(t, is.Equal(cfg.NetworkingConfig.EndpointsConfig[mode.NetworkName()], endpoint))
		// The caller's request must stay untouched: the resolution works on
		// copies, as a serializing transport would enforce.
		assert.Check(t, is.Equal(string(hostConfig.NetworkMode), ""))
		assert.Check(t, is.Equal(networking.EndpointsConfig[network.NetworkDefault], endpoint))
		assert.Check(t, is.Len(networking.EndpointsConfig, 1))
	})

	t.Run("an explicit network mode is passed through untouched", func(t *testing.T) {
		hostConfig := &container.HostConfig{NetworkMode: "host"}
		config := &container.Config{Image: "busybox"}
		cfg := backendCreateConfig(runtimev0.ContainerCreateRequest{
			Name:       "job",
			Config:     config,
			HostConfig: hostConfig,
		})
		assert.Check(t, is.Equal(cfg.HostConfig, hostConfig))
		assert.Check(t, is.Equal(cfg.Config, config))
		assert.Check(t, is.Equal(string(cfg.HostConfig.NetworkMode), "host"))
	})
}
