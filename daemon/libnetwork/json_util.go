package libnetwork

import (
	"encoding/json"
	"fmt"
)

// unmarshalJSONField unmarshals the given field of m into dst. It is used by
// the various UnmarshalJSON methods in this package that decode from an
// intermediate map[string]any.
func unmarshalJSONField(m map[string]any, field string, dst any) error {
	b, err := json.Marshal(m[field])
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", field, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", field, err)
	}
	return nil
}
