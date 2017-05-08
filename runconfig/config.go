package runconfig

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/volume"
	"github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
)

// ContainerDecoder implements httputils.ContainerDecoder
// calling DecodeContainerConfig.
type ContainerDecoder struct{}

// DecodeConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeConfig(src io.Reader) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	return DecodeContainerConfig(src)
}

// DecodeHostConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeHostConfig(src io.Reader) (*container.HostConfig, error) {
	return DecodeHostConfig(src)
}

// DecodeContainerConfig decodes a json encoded config into a ContainerConfigWrapper
// struct and returns both a Config and a HostConfig struct
// Be aware this function is not checking whether the resulted structs are nil,
// it's your business to do so
func DecodeContainerConfig(src io.Reader) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
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

		// Now validate all the volumes and binds
		if err := validateVolumesAndBindSettings(w.Config, hc); err != nil {
			return nil, nil, nil, err
		}
	}

	// Certain parameters need daemon-side validation that cannot be done
	// on the client, as only the daemon knows what is valid for the platform.
	if err := ValidateNetMode(w.Config, hc); err != nil {
		return nil, nil, nil, err
	}

	// Validate isolation
	if err := ValidateIsolation(hc); err != nil {
		return nil, nil, nil, err
	}

	// Validate QoS
	if err := ValidateQoS(hc); err != nil {
		return nil, nil, nil, err
	}
	return w.Config, hc, w.NetworkingConfig, nil
}

// validateVolumesAndBindSettings validates each of the volumes and bind settings
// passed by the caller to ensure they are valid.
func validateVolumesAndBindSettings(c *container.Config, hc *container.HostConfig) error {

	// Ensure all volumes and binds are valid.
	for spec := range c.Volumes {
		if _, err := volume.ParseMountSpec(spec, hc.VolumeDriver); err != nil {
			return fmt.Errorf("Invalid volume spec %q: %v", spec, err)
		}
	}
	for _, spec := range hc.Binds {
		if _, err := volume.ParseMountSpec(spec, hc.VolumeDriver); err != nil {
			return fmt.Errorf("Invalid bind mount spec %q: %v", spec, err)
		}
	}

	return nil
}
