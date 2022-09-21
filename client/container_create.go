package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"
	"path"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type configWrapper struct {
	*container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

// ContainerCreate creates a new container based on the given configuration.
// It can be associated with a name, but it's not mandatory.
func (cli *Client) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	var response container.CreateResponse

	if err := cli.NewVersionError("1.25", "stop timeout"); config != nil && config.StopTimeout != nil && err != nil {
		return response, err
	}
	if err := cli.NewVersionError("1.41", "specify container image platform"); platform != nil && err != nil {
		return response, err
	}

	if hostConfig != nil {
		if versions.LessThan(cli.ClientVersion(), "1.25") {
			// When using API 1.24 and under, the client is responsible for removing the container
			hostConfig.AutoRemove = false
		}
		if versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.42") || versions.LessThan(cli.ClientVersion(), "1.40") {
			// KernelMemory was added in API 1.40, and deprecated in API 1.42
			hostConfig.KernelMemory = 0
		}
		if platform != nil && platform.OS == "linux" && versions.LessThan(cli.ClientVersion(), "1.42") {
			// When using API under 1.42, the Linux daemon doesn't respect the ConsoleSize
			hostConfig.ConsoleSize = [2]uint{0, 0}
		}
	}

	query := url.Values{}
	if p := formatPlatform(platform); p != "" {
		query.Set("platform", p)
	}

	if containerName != "" {
		query.Set("name", containerName)
	}

	body := configWrapper{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}

	serverResp, err := cli.post(ctx, "/containers/create", query, body, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}

// formatPlatform returns a formatted string representing platform (e.g. linux/arm/v7).
//
// Similar to containerd's platforms.Format(), but does allow components to be
// omitted (e.g. pass "architecture" only, without "os":
// https://github.com/containerd/containerd/blob/v1.5.2/platforms/platforms.go#L243-L263
func formatPlatform(platform *specs.Platform) string {
	if platform == nil {
		return ""
	}
	return path.Join(platform.OS, platform.Architecture, platform.Variant)
}
