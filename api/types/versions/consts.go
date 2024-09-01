package versions

// Default version of Current REST API
const Default = "1.44"

// Min is the minimum API version that can be supported
// by the API server, specified as "major.minor". Note that the daemon
// may be configured with a different minimum API version, as returned
// in [github.com/docker/docker/api/types.Version.MinAPIVersion].
//
// API requests for API versions lower than the configured version produce
// an error.
const Min = "1.24"
