package build

import(
	"encoding/json"
)

// Filters describes a predicate for an API request.
//
// Each entry in the map is a filter term.
// Each term is evaluated against the set of values.
// A filter term is satisfied if any one of the values in the set is a match.
// An item matches the filters when all terms are satisfied.
//
// Like all other map types in Go, the zero value is empty and read-only.
type Filters map[string]map[string]bool

// PruneRequest contains the request body for POST /build/prune
//
// This struct is used for API version 1.53 and later.
// Earlier API versions use query parameters instead.
type BuildCachePruneRequest struct {
	All           bool				`json:"all,omitempty"`
	ReservedSpace int64				`json:"reservedSpace,omitempty"`
	MaxUsedSpace  int64				`json:"maxUsedSpace,omitempty"`
	MinFreeSpace  int64				`json:"minFreeSpace,omitempty"`
	Filters       json.RawMessage	`json:"filters,omitempty"`
}
