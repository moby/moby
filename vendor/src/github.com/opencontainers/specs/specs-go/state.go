package specs

// State holds information about the runtime state of the container.
type State struct {
	// Version is the version of the specification that is supported.
	Version string `json:"version"`
	// ID is the container ID
	ID string `json:"id"`
	// Status is the runtime state of the container.
	Status string `json:"status"`
	// Pid is the process id for the container's main process.
	Pid int `json:"pid"`
	// BundlePath is the path to the container's bundle directory.
	BundlePath string `json:"bundlePath"`
	// Annotations are the annotations associated with the container.
	Annotations map[string]string `json:"annotations"`
}
