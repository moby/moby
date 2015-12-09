package specs

// State holds information about the runtime state of the container.
// This information will be stored in a file called `state.json`.
// The location of this file will be operating system specific. On Linux
// it will be in `/run/opencontainers/runc/<containerID>/state.json`
type State struct {
	// Version is the version of the specification that is supported.
	Version string `json:"version"`
	// ID is the container ID
	ID string `json:"id"`
	// Pid is the process id for the container's main process.
	Pid int `json:"pid"`
	// BundlePath is the path to the container's bundle directory.
	BundlePath string `json:"bundlePath"`
}
