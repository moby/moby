package runconfig

import (
	"encoding/json"
	"io"
	"runtime"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/pkg/sysinfo"
)

// DecodeCreateRequest decodes a json encoded [container.CreateRequest] struct
// and performs some validation. Certain parameters need daemon-side validation
// that cannot be done on the client, as only the daemon knows what is valid
// for the platform.
func DecodeCreateRequest(src io.Reader, si *sysinfo.SysInfo) (container.CreateRequest, error) {
	w, err := decodeCreateRequest(src)
	if err != nil {
		return container.CreateRequest{}, err
	}
	if err := validateCreateRequest(w, si); err != nil {
		return container.CreateRequest{}, err
	}
	return w, nil
}

// decodeCreateRequest decodes a json encoded [container.CreateRequest] struct
// and sets some defaults.
func decodeCreateRequest(src io.Reader) (container.CreateRequest, error) {
	// TODO(thaJeztah): replace with httputils.ReadJSON ?
	var w container.CreateRequest
	if err := loadJSON(src, &w); err != nil {
		return container.CreateRequest{}, err
	}
	if w.Config == nil {
		return container.CreateRequest{}, validationError("config cannot be empty in order to create a container")
	}
	if w.Config == nil {
		w.Config.Volumes = make(map[string]struct{})
	}
	if w.HostConfig == nil {
		w.HostConfig = &container.HostConfig{}
	}
	// Make sure NetworkMode has an acceptable value. We do this to ensure
	// backwards compatible API behavior.
	//
	// TODO(thaJeztah): platform check may be redundant, as other code-paths execute this unconditionally. Also check if this code is still needed here, or already handled elsewhere.
	if runtime.GOOS != "windows" && w.HostConfig.NetworkMode == "" {
		w.HostConfig.NetworkMode = network.NetworkDefault
	}
	if w.NetworkingConfig == nil {
		w.NetworkingConfig = &network.NetworkingConfig{}
	}
	if w.NetworkingConfig.EndpointsConfig == nil {
		w.NetworkingConfig.EndpointsConfig = make(map[string]*network.EndpointSettings)
	}
	return w, nil
}

func validateCreateRequest(w container.CreateRequest, si *sysinfo.SysInfo) error {
	if err := validateNetMode(w.Config, w.HostConfig); err != nil {
		return err
	}
	if err := validateIsolation(w.HostConfig); err != nil {
		return err
	}
	if err := validateQoS(w.HostConfig); err != nil {
		return err
	}
	if err := validateResources(w.HostConfig, si); err != nil {
		return err
	}
	if err := validatePrivileged(w.HostConfig); err != nil {
		return err
	}
	if err := validateReadonlyRootfs(w.HostConfig); err != nil {
		return err
	}
	return nil
}

// loadJSON is similar to api/server/httputils.ReadJSON()
func loadJSON(src io.Reader, out any) error {
	dec := json.NewDecoder(src)
	if err := dec.Decode(&out); err != nil {
		// invalidJSONError allows unwrapping the error to detect io.EOF etc.
		return invalidJSONError{error: err}
	}
	if dec.More() {
		return validationError("unexpected content after JSON")
	}
	return nil
}
