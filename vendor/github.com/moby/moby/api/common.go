package api

// Common constants for daemon and client.
const (
	// DefaultVersion of the current REST API.
	DefaultVersion = "1.52"

	// MinSupportedAPIVersion is the minimum API version that can be supported
	// by the API server, specified as "major.minor". Note that the daemon
	// may be configured with a different minimum API version, as returned
	// in [github.com/moby/moby/api/types.Version.MinAPIVersion].
	//
	// API requests for API versions lower than the configured version produce
	// an error.
	MinSupportedAPIVersion = "1.24"

	// NoBaseImageSpecifier is the symbol used by the FROM
	// command to specify that no base image is to be used.
	//
	// Deprecated: this const is no longer used and will be removed in the next release.
	NoBaseImageSpecifier = "scratch"
)
