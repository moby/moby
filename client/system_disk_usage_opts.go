package client

import "github.com/moby/moby/api/types/system"

// DiskUsageOptions holds parameters for system disk usage query.
type DiskUsageOptions struct {
	// Types specifies what object types to include in the response. If empty,
	// all object types are returned.
	Types []system.DiskUsageObject
}
