package runconfig // import "github.com/docker/docker/runconfig"

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/sysinfo"
)

// ContainerDecoder implements httputils.ContainerDecoder
// calling DecodeContainerConfig.
type ContainerDecoder struct{}

// DecodeConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeConfig(src io.Reader) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	return decodeContainerConfig(src)
}

// DecodeHostConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeHostConfig(src io.Reader) (*container.HostConfig, error) {
	return decodeHostConfig(src)
}

// decodeContainerConfig decodes a json encoded config into a ContainerConfigWrapper
// struct and returns both a Config and a HostConfig struct
// Be aware this function is not checking whether the resulted structs are nil,
// it's your business to do so
func decodeContainerConfig(src io.Reader) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	var w ContainerConfigWrapper

	decoder := json.NewDecoder(src)
	if err := decoder.Decode(&w); err != nil {
		return nil, nil, nil, err
	}

	hc := w.getHostConfig()

	// Perform platform-specific processing of Volumes and Binds.
	if w.Config != nil && hc != nil {

		// Initialize the volumes map if currently nil
		if w.Config.Volumes == nil {
			w.Config.Volumes = make(map[string]struct{})
		}
	}

	// Certain parameters need daemon-side validation that cannot be done
	// on the client, as only the daemon knows what is valid for the platform.
	if err := validateNetMode(w.Config, hc); err != nil {
		return nil, nil, nil, err
	}

	// Validate isolation
	if err := validateIsolation(hc); err != nil {
		return nil, nil, nil, err
	}

	// Validate QoS
	if err := validateQoS(hc); err != nil {
		return nil, nil, nil, err
	}

	// Validate Resources
	if err := validateResources(hc, sysinfo.New(true)); err != nil {
		return nil, nil, nil, err
	}

	// Validate Privileged
	if err := validatePrivileged(hc); err != nil {
		return nil, nil, nil, err
	}

	// Validate ReadonlyRootfs
	if err := validateReadonlyRootfs(hc); err != nil {
		return nil, nil, nil, err
	}

	return w.Config, hc, w.NetworkingConfig, nil
}
