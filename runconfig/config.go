package runconfig // import "github.com/docker/docker/runconfig"

import (
	"encoding/json"
	"io"
	"runtime"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/sysinfo"
)

// ContainerConfigWrapper is a Config wrapper that holds the container Config (portable)
// and the corresponding HostConfig (non-portable).
//
// Deprecated: use [container.CreateRequest].
type ContainerConfigWrapper = container.CreateRequest

// ContainerDecoder implements httputils.ContainerDecoder
// calling DecodeContainerConfig.
type ContainerDecoder struct {
	GetSysInfo func() *sysinfo.SysInfo
}

// DecodeConfig makes ContainerDecoder to implement httputils.ContainerDecoder
func (r ContainerDecoder) DecodeConfig(src io.Reader) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error) {
	var si *sysinfo.SysInfo
	if r.GetSysInfo != nil {
		si = r.GetSysInfo()
	} else {
		si = sysinfo.New()
	}

	return decodeContainerConfig(src, si)
}

// decodeContainerConfig decodes a json encoded config into a ContainerConfigWrapper
// struct and returns both a Config and a HostConfig struct, and performs some
// validation. Certain parameters need daemon-side validation that cannot be done
// on the client, as only the daemon knows what is valid for the platform.
// Be aware this function is not checking whether the resulted structs are nil,
// it's your business to do so
func decodeContainerConfig(src io.Reader, si *sysinfo.SysInfo) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error) {
	var w container.CreateRequest
	if err := loadJSON(src, &w); err != nil {
		return nil, nil, nil, err
	}

	hc := w.HostConfig
	if hc == nil {
		// We may not be passed a host config, such as in the case of docker commit
		return w.Config, hc, w.NetworkingConfig, nil
	}

	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards compatible API behavior.
	//
	// TODO(thaJeztah): platform check may be redundant, as other code-paths execute this unconditionally. Also check if this code is still needed here, or already handled elsewhere.
	if runtime.GOOS != "windows" && hc.NetworkMode == "" {
		hc.NetworkMode = network.NetworkDefault
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

// loadJSON is similar to api/server/httputils.ReadJSON()
func loadJSON(src io.Reader, out interface{}) error {
	dec := json.NewDecoder(src)
	if err := dec.Decode(&out); err != nil {
		return invalidJSONError{Err: err}
	}
	if dec.More() {
		return validationError("unexpected content after JSON")
	}
	return nil
}
