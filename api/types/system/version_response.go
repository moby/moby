package system

// VersionResponse contains information about the Docker server host.
// GET "/version"
type VersionResponse struct {
	// Platform is the platform (product name) the server is running on.
	Platform PlatformInfo `json:",omitempty"`

	// Version is the version of the daemon.
	Version string

	// APIVersion is the highest API version supported by the server.
	APIVersion string `json:"ApiVersion"`

	// MinAPIVersion is the minimum API version the server supports.
	MinAPIVersion string `json:"MinAPIVersion,omitempty"`

	// Os is the operating system the server runs on.
	Os string

	// Arch is the hardware architecture the server runs on.
	Arch string

	// Components contains version information for the components making
	// up the server. Information in this field is for informational
	// purposes, and not part of the API contract.
	Components []ComponentVersion `json:",omitempty"`

	// The following fields are deprecated, they relate to the Engine component and are kept for backwards compatibility

	GitCommit     string `json:",omitempty"`
	GoVersion     string `json:",omitempty"`
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}

// PlatformInfo holds information about the platform (product name) the
// server is running on.
type PlatformInfo struct {
	// Name is the name of the platform (for example, "Docker Engine - Community",
	// or "Docker Desktop 4.49.0 (208003)")
	Name string
}

// ComponentVersion describes the version information for a specific component.
type ComponentVersion struct {
	Name    string
	Version string

	// Details contains Key/value pairs of strings with additional information
	// about the component. These values are intended for informational purposes
	// only, and their content is not defined, and not part of the API
	// specification.
	//
	// These messages can be printed by the client as information to the user.
	Details map[string]string `json:",omitempty"`
}
