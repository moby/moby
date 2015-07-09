package runconfig

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/pkg/nat"
)

// Entrypoint encapsulates the container entrypoint.
// It might be represented as a string or an array of strings.
// We need to override the json decoder to accept both options.
// The JSON decoder will fail if the api sends an string and
//  we try to decode it into an array of string.
type Entrypoint struct {
	parts []string
}

// MarshalJSON Marshals (or serializes) the Entrypoint into the json format.
// This method is needed to implement json.Marshaller.
func (e *Entrypoint) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte{}, nil
	}
	return json.Marshal(e.Slice())
}

// UnmarshalJSON decodes the entrypoint whether it's a string or an array of strings.
// This method is needed to implement json.Unmarshaler.
func (e *Entrypoint) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := make([]string, 0, 1)
	if err := json.Unmarshal(b, &p); err != nil {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p = append(p, s)
	}
	e.parts = p
	return nil
}

// Len returns the number of parts of the Entrypoint.
func (e *Entrypoint) Len() int {
	if e == nil {
		return 0
	}
	return len(e.parts)
}

// Slice gets the parts of the Entrypoint as a Slice of string.
func (e *Entrypoint) Slice() []string {
	if e == nil {
		return nil
	}
	return e.parts
}

// NewEntrypoint creates an Entrypoint based on the specified parts (as strings).
func NewEntrypoint(parts ...string) *Entrypoint {
	return &Entrypoint{parts}
}

// Command encapsulates the container command.
// It might be represented as a string or an array of strings.
// We need to override the json decoder to accept both options.
// The JSON decoder will fail if the api sends an string and
//  we try to decode it into an array of string.
type Command struct {
	parts []string
}

// ToString gets a string representing a Command.
func (e *Command) ToString() string {
	return strings.Join(e.parts, " ")
}

// MarshalJSON Marshals (or serializes) the Command into the json format.
// This method is needed to implement json.Marshaller.
func (e *Command) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte{}, nil
	}
	return json.Marshal(e.Slice())
}

// UnmarshalJSON decodes the entrypoint whether it's a string or an array of strings.
// This method is needed to implement json.Unmarshaler.
func (e *Command) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	p := make([]string, 0, 1)
	if err := json.Unmarshal(b, &p); err != nil {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p = append(p, s)
	}
	e.parts = p
	return nil
}

// Len returns the number of parts of the Entrypoint.
func (e *Command) Len() int {
	if e == nil {
		return 0
	}
	return len(e.parts)
}

// Slice gets the parts of the Entrypoint as a Slice of string.
func (e *Command) Slice() []string {
	if e == nil {
		return nil
	}
	return e.parts
}

// NewCommand creates a Command based on the specified parts (as strings).
func NewCommand(parts ...string) *Command {
	return &Command{parts}
}

// Config contains the configuration data about a container.
// It should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
type Config struct {
	Hostname        string                // Hostname
	Domainname      string                // Domainname
	User            string                // User that will run the command(s) inside the container
	AttachStdin     bool                  // Attach the standard input, makes possible user interaction
	AttachStdout    bool                  // Attach the standard output
	AttachStderr    bool                  // Attach the standard error
	ExposedPorts    map[nat.Port]struct{} // List of exposed ports
	PublishService  string                // Name of the network service exposed by the container
	Tty             bool                  // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool                  // Open stdin
	StdinOnce       bool                  // If true, close stdin after the 1 attached client disconnects.
	Env             []string              // List of environment variable to set in the container
	Cmd             *Command              // Command to run when starting the container
	Image           string                // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes         map[string]struct{}   // List of volumes (mounts) used for the container
	VolumeDriver    string                // Name of the volume driver used to mount volumes
	WorkingDir      string                // Current directory (PWD) in the command will be launched
	Entrypoint      *Entrypoint           // Entrypoint to run when starting the container
	NetworkDisabled bool                  // Is network disabled
	MacAddress      string                // Mac Address of the container
	OnBuild         []string              // ONBUILD metadata that were defined on the image Dockerfile
	Labels          map[string]string     // List of labels set to this container
}

// DecodeContainerConfig decodes a json encoded config into a ContainerConfigWrapper
// struct and returns both a Config and an HostConfig struct
// Be aware this function is not checking whether the resulted structs are nil,
// it's your business to do so
func DecodeContainerConfig(src io.Reader) (*Config, *HostConfig, error) {
	decoder := json.NewDecoder(src)

	var w ContainerConfigWrapper
	if err := decoder.Decode(&w); err != nil {
		return nil, nil, err
	}

	hc := w.getHostConfig()

	// Certain parameters need daemon-side validation that cannot be done
	// on the client, as only the daemon knows what is valid for the platform.
	if err := ValidateNetMode(w.Config, hc); err != nil {
		return nil, nil, err
	}

	return w.Config, hc, nil
}
