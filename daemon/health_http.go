// +build !linux

package daemon

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/container"
)

func (p *httpProbe) run(ctx context.Context, d *Daemon, container *container.Container) (*types.HealthcheckResult, error) {
	httpSlice := strslice.StrSlice(container.Config.Healthcheck.Test)[1:]

	endpoint := "/"
	if len(httpSlice) > 0 {
		endpoint = httpSlice[0]
	}

	address := ""
	if container.HostConfig.NetworkMode.IsHost() {
		address = "127.0.0.1"
	} else {
		for _, n := range container.NetworkSettings.Networks {
			if n.IPAddress != "" {
				address = n.IPAddress
				break
			}
		}
	}
	if address == "" {
		return nil, fmt.Errorf("unable to find network with IP address")
	}

	return httpHealthcheck(address, endpoint, container.Config.Healthcheck.Timeout)
}
