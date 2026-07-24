package swarm

// NodeUpdateRequest contains the request body for POST /nodes/{id}/update
//
// This struct is used for API version 1.53 and later.
// Earlier API versions use query parameters for version instead.
type NodeUpdateRequest struct {
	Version uint64   `json:"version"`
	Spec    NodeSpec `json:"spec"`
}
