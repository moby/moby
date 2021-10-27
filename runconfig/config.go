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
type ContainerDecoder struct {
	GetSysInfo func() *sysinfo.SysInfo
}

// DecodeConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeConfig(src io.Reader) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	var si *sysinfo.SysInfo
	if r.GetSysInfo != nil {
		si = r.GetSysInfo()
	} else {
		si = sysinfo.New()
	}

	return decodeContainerConfig(src, si)
}

// DecodeHostConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeHostConfig(src io.Reader) (*container.HostConfig, error) {
	return decodeHostConfig(src)
}

// decodeContainerConfig decodes a json encoded config into a ContainerConfigWrapper
// struct and returns both a Config and a HostConfig struct, and performs some
// validation. Certain parameters need daemon-side validation that cannot be done
// on the client, as only the daemon knows what is valid for the platform.
// Be aware this function is not checking whether the resulted structs are nil,
// it's your business to do so
func decodeContainerConfig(src io.Reader, si *sysinfo.SysInfo) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	var w ContainerConfigWrapper
	if err := json.NewDecoder(src).Decode(&w); err != nil {
		return nil, nil, nil, err
	}

	hc := w.getHostConfig()
	if hc == nil {
		// We may not be passed a host config, such as in the case of docker commit
		return w.Config, hc, w.NetworkingConfig, nil
	}
	if err := validateNetMode(w.Config, hc); err != nil {
		return nil, nil, nil, err
	}
	if err := validateIsolation(hc); err != nil {
		return nil, nil, nil, err
	}
	if err := validateQoS(hc); err != nil {
		return nil, nil, nil, err
	}
	if err := validateResources(hc, si); err != nil {
		return nil, nil, nil, err
	}
	if err := validatePrivileged(hc); err != nil {
		return nil, nil, nil, err
	}
	if err := validateReadonlyRootfs(hc); err != nil {
		return nil, nil, nil, err
	}
	if w.Config != nil && w.Config.Volumes == nil {
		w.Config.Volumes = make(map[string]struct{})
	}
	return w.Config, hc, w.NetworkingConfig, nil
}
