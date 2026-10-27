package daemon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/internal/runconfig"
	networkSettings "github.com/moby/moby/v2/daemon/network"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/errdefs"
	containerruntimev0 "github.com/moby/moby/v2/extpoints/container/runtime/v0"
	"github.com/moby/moby/v2/pkg/sysinfo"
)

// containerRuntime provides the container runtime point from the daemon.
type containerRuntime struct {
	d *Daemon
}

func (r containerRuntime) CreateFromAPI(ctx context.Context, req *containerruntimev0.CreateFromAPIRequest) (*containerruntimev0.CreateFromAPIReply, error) {
	sysInfo, err := r.d.RawSysInfo()
	if err != nil {
		return nil, err
	}

	cfg, err := backendCreateConfig(req, sysInfo)
	if err != nil {
		return nil, err
	}

	resp, err := r.d.ContainerCreate(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &containerruntimev0.CreateFromAPIReply{ID: resp.ID, Warnings: resp.Warnings}, nil
}

// backendCreateConfig decodes and validates the point's create request into
// the backend form.
// It resolves the OS-independent "default" network mode to the OS's default
// network, as the HTTP API route does before calling the backend; otherwise
// the container fails to start looking up a network named "default".
func backendCreateConfig(req *containerruntimev0.CreateFromAPIRequest, sysInfo *sysinfo.SysInfo) (backend.ContainerCreateConfig, error) {
	var cr container.CreateRequest
	if err := decodeConfig("Config", req.Config, &cr.Config); err != nil {
		return backend.ContainerCreateConfig{}, err
	}

	if err := decodeConfig("HostConfig", req.HostConfig, &cr.HostConfig); err != nil {
		return backend.ContainerCreateConfig{}, err
	}

	if err := decodeConfig("NetworkingConfig", req.NetworkingConfig, &cr.NetworkingConfig); err != nil {
		return backend.ContainerCreateConfig{}, err
	}

	cr, err := runconfig.NormalizeCreateRequest(cr, sysInfo)
	if err != nil {
		return backend.ContainerCreateConfig{}, err
	}

	hostConfig, networkingConfig := cr.HostConfig, cr.NetworkingConfig
	if hostConfig.NetworkMode == "" || hostConfig.NetworkMode.IsDefault() {
		hostConfig.NetworkMode = networkSettings.DefaultNetwork
		if nw, ok := networkingConfig.EndpointsConfig[network.NetworkDefault]; ok {
			networkingConfig.EndpointsConfig[hostConfig.NetworkMode.NetworkName()] = nw
			delete(networkingConfig.EndpointsConfig, network.NetworkDefault)
		}
	}

	return backend.ContainerCreateConfig{
		Name:             req.Name,
		Config:           cr.Config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}, nil
}

// decodeConfig decodes a JSON-encoded config into out, leaving out unset when
// data is empty.
func decodeConfig(field string, data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}

	if err := json.Unmarshal(data, out); err != nil {
		return errdefs.InvalidParameter(fmt.Errorf("decode %s: %w", field, err))
	}

	return nil
}

func (r containerRuntime) Start(ctx context.Context, req *containerruntimev0.StartRequest) error {
	return r.d.ContainerStart(ctx, req.Container, "", "")
}

func (r containerRuntime) Stop(ctx context.Context, req *containerruntimev0.StopRequest) error {
	return r.d.ContainerStop(ctx, req.Container, backend.ContainerStopOptions{})
}

func (r containerRuntime) Remove(_ context.Context, req *containerruntimev0.RemoveRequest) error {
	return r.d.ContainerRm(req.Container, &backend.ContainerRmConfig{})
}
