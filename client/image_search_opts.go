package client

import (
	"context"

	"github.com/moby/moby/api/types/registry"
)

// ImageSearchResult wraps results returned by ImageSearch.
type ImageSearchResult struct {
	Items []registry.SearchResult
}

// ImageSearchOptions holds parameters to search images with.
type ImageSearchOptions struct {
	RegistryAuth string

	// PrivilegeFunc is a function that clients can supply to retry operations
	// after getting an authorization error. This function returns the registry
	// authentication header value in base64 encoded format, or an error if the
	// privilege request fails.
	//
	// For details, refer to [github.com/moby/moby/api/types/registry.RequestAuthConfig].
	PrivilegeFunc func(context.Context) (string, error)
	Filters       Filters
	Limit         int
}
