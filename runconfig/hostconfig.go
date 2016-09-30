package runconfig

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types/container"
)

// DecodeHostConfig creates a HostConfig based on the specified Reader.
// It assumes the content of the reader will be JSON, and decodes it.
func DecodeHostConfig(src io.Reader) (*container.HostConfig, error) {
	decoder := json.NewDecoder(src)

	var w ContainerConfigWrapper
	if err := decoder.Decode(&w); err != nil {
		return nil, err
	}

	hc := w.getHostConfig()
	return hc, nil
}

// SetDefaultNetModeIfBlank changes the NetworkMode in a HostConfig structure
// to default if it is not populated. This ensures backwards compatibility after
// the validation of the network mode was moved from the docker CLI to the
// docker daemon.
func SetDefaultNetModeIfBlank(hc *container.HostConfig) *container.HostConfig {
	if hc != nil {
		if hc.NetworkMode == container.NetworkMode("") {
			hc.NetworkMode = container.NetworkMode("default")
		}
	}
	return hc
}
