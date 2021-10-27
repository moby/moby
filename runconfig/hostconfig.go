package runconfig // import "github.com/docker/docker/runconfig"

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// DecodeHostConfig creates a HostConfig based on the specified Reader.
// It assumes the content of the reader will be JSON, and decodes it.
func decodeHostConfig(src io.Reader) (*container.HostConfig, error) {
	var w ContainerConfigWrapper
	if err := json.NewDecoder(src).Decode(&w); err != nil {
		return nil, err
	}
	return w.getHostConfig(), nil
}

// SetDefaultNetModeIfBlank changes the NetworkMode in a HostConfig structure
// to default if it is not populated. This ensures backwards compatibility after
// the validation of the network mode was moved from the docker CLI to the
// docker daemon.
func SetDefaultNetModeIfBlank(hc *container.HostConfig) {
	if hc != nil && hc.NetworkMode == "" {
		hc.NetworkMode = "default"
	}
}

// validateNetContainerMode ensures that the various combinations of requested
// network settings wrt container mode are valid.
func validateNetContainerMode(c *container.Config, hc *container.HostConfig) error {
	parts := strings.Split(string(hc.NetworkMode), ":")
	if parts[0] == "container" {
		if len(parts) < 2 || parts[1] == "" {
			return validationError("Invalid network mode: invalid container format container:<name|id>")
		}
	}

	if hc.NetworkMode.IsContainer() && c.Hostname != "" {
		return ErrConflictNetworkHostname
	}

	if hc.NetworkMode.IsContainer() && len(hc.Links) > 0 {
		return ErrConflictContainerNetworkAndLinks
	}

	if hc.NetworkMode.IsContainer() && len(hc.DNS) > 0 {
		return ErrConflictNetworkAndDNS
	}

	if hc.NetworkMode.IsContainer() && len(hc.ExtraHosts) > 0 {
		return ErrConflictNetworkHosts
	}

	if (hc.NetworkMode.IsContainer() || hc.NetworkMode.IsHost()) && c.MacAddress != "" {
		return ErrConflictContainerNetworkAndMac
	}

	if hc.NetworkMode.IsContainer() && (len(hc.PortBindings) > 0 || hc.PublishAllPorts) {
		return ErrConflictNetworkPublishPorts
	}

	if hc.NetworkMode.IsContainer() && len(c.ExposedPorts) > 0 {
		return ErrConflictNetworkExposePorts
	}
	return nil
}
