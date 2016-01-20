package container

import (
	"github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-connections/nat"
)

// Config contains the configuration data about a container.
// It should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
// All fields added to this struct must be marked `omitempty` to keep getting
// predictable hashes from the old `v1Compatibility` configuration.
type Config struct {
	Hostname        string                `json:",omitempty"` // Hostname
	Domainname      string                `json:",omitempty"` // Domainname
	User            string                `json:",omitempty"` // User that will run the command(s) inside the container
	AttachStdin     bool                  `json:",omitempty"` // Attach the standard input, makes possible user interaction
	AttachStdout    bool                  `json:",omitempty"` // Attach the standard output
	AttachStderr    bool                  `json:",omitempty"` // Attach the standard error
	ExposedPorts    map[nat.Port]struct{} `json:",omitempty"` // List of exposed ports
	PublishService  string                `json:",omitempty"` // Name of the network service exposed by the container
	Tty             bool                  `json:",omitempty"` // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool                  `json:",omitempty"` // Open stdin
	StdinOnce       bool                  `json:",omitempty"` // If true, close stdin after the 1 attached client disconnects.
	Env             []string              `json:",omitempty"` // List of environment variable to set in the container
	Cmd             *strslice.StrSlice    `json:",omitempty"` // Command to run when starting the container
	ArgsEscaped     bool                  `json:",omitempty"` // True if command is already escaped (Windows specific)
	Image           string                `json:",omitempty"` // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes         map[string]struct{}   `json:",omitempty"` // List of volumes (mounts) used for the container
	WorkingDir      string                `json:",omitempty"` // Current directory (PWD) in the command will be launched
	Entrypoint      *strslice.StrSlice    `json:",omitempty"` // Entrypoint to run when starting the container
	NetworkDisabled bool                  `json:",omitempty"` // Is network disabled
	MacAddress      string                `json:",omitempty"` // Mac Address of the container
	OnBuild         []string              `json:",omitempty"` // ONBUILD metadata that were defined on the image Dockerfile
	Labels          map[string]string     `json:",omitempty"` // List of labels set to this container
	StopSignal      string                `json:",omitempty"` // Signal to stop a container
}
