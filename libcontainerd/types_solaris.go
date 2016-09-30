package libcontainerd

// Process contains information to start a specific application inside the container.
type Process struct {
	// Terminal creates an interactive terminal for the container.
	Terminal bool `json:"terminal"`
	// Args specifies the binary and arguments for the application to execute.
	Args []string `json:"args"`
}

// Stats contains a stats properties from containerd.
type Stats struct{}

// Summary contains a container summary from containerd
type Summary struct{}

// StateInfo contains description about the new state container has entered.
type StateInfo struct {
	CommonStateInfo

	// Platform specific StateInfo
}

// Resources defines updatable container resource values.
type Resources struct{}
