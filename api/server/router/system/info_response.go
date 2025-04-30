// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.23

package system

import (
	"encoding/json"

	"github.com/docker/docker/api/types/system"
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
func (sc *infoResponse) MarshalJSON() ([]byte, error) {
	type tmp *system.Info
	base, err := json.Marshal((tmp)(sc.Info))
	if err != nil {
		return nil, err
	}
	if len(sc.extraFields) == 0 {
		return base, nil
	}
	var merged map[string]any
	_ = json.Unmarshal(base, &merged)

	for k, v := range sc.extraFields {
		merged[k] = v
	}
	return json.Marshal(merged)
}
