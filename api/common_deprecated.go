package api

import "github.com/docker/docker/api/types/versions"

// Common constants for daemon and client.
const (
	// DefaultVersion of Current REST API.
	//
	// Deprecated: use [versions.Default].
	DefaultVersion = versions.Default

	// MinSupportedAPIVersion is the minimum API version that can be supported
	// by the API server, specified as "major.minor". Note that the daemon
	// may be configured with a different minimum API version, as returned
	// in [github.com/docker/docker/api/types.Version.MinAPIVersion].
	//
	// API requests for API versions lower than the configured version produce
	// an error.
	//
	// Deprecated: use [versions.Min].
	MinSupportedAPIVersion = versions.Min
)
