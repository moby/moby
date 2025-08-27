package system

import (
	"encoding/json"
	"maps"

	"github.com/moby/moby/api/types/system"
)

// infoResponse is a wrapper around [system.Info] with a custom
// marshal function for legacy fields.
type infoResponse struct {
	*system.Info

	// extraFields is for internal use to include deprecated fields on older API versions.
	extraFields map[string]any
}

// MarshalJSON implements a custom marshaler to include legacy fields
// in API responses.
func (ir *infoResponse) MarshalJSON() ([]byte, error) {
	type tmp *system.Info
	base, err := json.Marshal((tmp)(ir.Info))
	if err != nil {
		return nil, err
	}
	if len(ir.extraFields) == 0 && (ir.Info == nil || ir.Info.RegistryConfig == nil || len(ir.Info.RegistryConfig.ExtraFields) == 0) {
		return base, nil
	}
	var merged map[string]any
	_ = json.Unmarshal(base, &merged)

	// Merge top-level extraFields
	maps.Copy(merged, ir.extraFields)

	// Merge RegistryConfig.ExtraFields if present
	if ir.Info != nil && ir.Info.RegistryConfig != nil && len(ir.Info.RegistryConfig.ExtraFields) > 0 {
		if rc, ok := merged["RegistryConfig"].(map[string]any); ok {
			maps.Copy(rc, ir.Info.RegistryConfig.ExtraFields)
		}
	}

	return json.Marshal(merged)
}
