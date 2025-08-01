package image

import (
	"encoding/json"
	"maps"

	"github.com/moby/moby/api/types/image"
)

// legacyConfigFields defines legacy image-config fields to include in
// API responses on older API versions.
var legacyConfigFields = map[string]map[string]any{
	// Legacy fields for API v1.49 and lower. These fields are deprecated
	// and omitted in newer API versions; see https://github.com/moby/moby/pull/48457
	"v1.49": {
		"AttachStderr": false,
		"AttachStdin":  false,
		"AttachStdout": false,
		"Cmd":          nil,
		"Domainname":   "",
		"Entrypoint":   nil,
		"Env":          nil,
		"Hostname":     "",
		"Image":        "",
		"Labels":       nil,
		"OnBuild":      nil,
		"OpenStdin":    false,
		"StdinOnce":    false,
		"Tty":          false,
		"User":         "",
		"Volumes":      nil,
		"WorkingDir":   "",
	},
	// Legacy fields for current API versions (v1.50 and up). These fields
	// did not have an "omitempty" and were always included in the response,
	// even if not set; see https://github.com/moby/moby/issues/50134
	"current": {
		"Cmd":        nil,
		"Entrypoint": nil,
		"Env":        nil,
		"Labels":     nil,
		"OnBuild":    nil,
		"User":       "",
		"Volumes":    nil,
		"WorkingDir": "",
	},
}

// inspectCompatResponse is a wrapper around [image.InspectResponse] with a
// custom marshal function for legacy [api/types/container.Config} fields
// that have been removed, or did not have omitempty.
type inspectCompatResponse struct {
	*image.InspectResponse
	legacyConfig map[string]any
}

// MarshalJSON implements a custom marshaler to include legacy fields
// in API responses.
func (ir *inspectCompatResponse) MarshalJSON() ([]byte, error) {
	type tmp *image.InspectResponse
	base, err := json.Marshal((tmp)(ir.InspectResponse))
	if err != nil {
		return nil, err
	}
	if len(ir.legacyConfig) == 0 {
		return base, nil
	}

	type resp struct {
		*image.InspectResponse
		Config map[string]any
	}

	var merged resp
	err = json.Unmarshal(base, &merged)
	if err != nil {
		return base, nil
	}

	// prevent mutating legacyConfigFields.
	cfg := maps.Clone(ir.legacyConfig)
	maps.Copy(cfg, merged.Config)
	merged.Config = cfg
	return json.Marshal(merged)
}
