// Package v1p24 provides specific API types for the API version 1, patch 24.
package v1p24

import "github.com/docker/docker/api/types"

// Info is a backcompatibility struct for the API 1.24
type Info struct {
	*types.InfoBase
	ExecutionDriver string
	SecurityOptions []string
}
