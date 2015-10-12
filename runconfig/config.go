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

func (e *Entrypoint) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte{}, nil
	}
	return json.Marshal(e.Slice())
}

// UnmarshalJSON decoded the entrypoint whether it's a string or an array of strings.
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

func (e *Entrypoint) Len() int {
	if e == nil {
		return 0
	}
	return len(e.parts)
}

func (e *Entrypoint) Slice() []string {
	if e == nil {
		return nil
	}
	return e.parts
}

func NewEntrypoint(parts ...string) *Entrypoint {
	return &Entrypoint{parts}
}

type Command struct {
	parts []string
}

func (e *Command) ToString() string {
	return strings.Join(e.parts, " ")
}

func (e *Command) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte{}, nil
	}
	return json.Marshal(e.Slice())
}

// UnmarshalJSON decoded the entrypoint whether it's a string or an array of strings.
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

func (e *Command) Len() int {
	if e == nil {
		return 0
	}
	return len(e.parts)
}

func (e *Command) Slice() []string {
	if e == nil {
		return nil
	}
	return e.parts
}

func NewCommand(parts ...string) *Command {
	return &Command{parts}
}

// Note: the Config structure should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
// All fields added to this struct must be marked `omitempty` to keep getting
// predictable hashes from the old `v1Compatibility` configuration.
type Config struct {
	Hostname        string                // Hostname
	Domainname      string                // Domainname
	User            string                // User that will run the command(s) inside the container
	AttachStdin     bool                  // Attach the standard input, makes possible user interaction
	AttachStdout    bool                  // Attach the standard output
	AttachStderr    bool                  // Attach the standard error
	ExposedPorts    map[nat.Port]struct{} `json:",omitempty"` // List of exposed ports
	PublishService  string                `json:",omitempty"` // Name of the network service exposed by the container
	Tty             bool                  // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool                  // Open stdin
	StdinOnce       bool                  // If true, close stdin after the 1 attached client disconnects.
	Env             []string              // List of environment variable to set in the container
	Cmd             *Command              // Command to run when starting the container
	Image           string                // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes         map[string]struct{}   // List of volumes (mounts) used for the container
	VolumeDriver    string                `json:",omitempty"` // Name of the volume driver used to mount volumes
	WorkingDir      string                // Current directory (PWD) in the command will be launched
	Entrypoint      *Entrypoint           // Entrypoint to run when starting the container
	NetworkDisabled bool                  `json:",omitempty"` // Is network disabled
	MacAddress      string                `json:",omitempty"` // Mac Address of the container
	OnBuild         []string              // ONBUILD metadata that were defined on the image Dockerfile
	Labels          map[string]string     // List of labels set to this container
}

type ContainerConfigWrapper struct {
	*Config
	InnerHostConfig *HostConfig `json:"HostConfig,omitempty"`
	Cpuset          string      `json:",omitempty"` // Deprecated. Exported for backwards compatibility.
	*HostConfig                 // Deprecated. Exported to read attrubutes from json that are not in the inner host config structure.

}

func (w *ContainerConfigWrapper) GetHostConfig() *HostConfig {
	hc := w.HostConfig

	if hc == nil && w.InnerHostConfig != nil {
		hc = w.InnerHostConfig
	} else if w.InnerHostConfig != nil {
		if hc.Memory != 0 && w.InnerHostConfig.Memory == 0 {
			w.InnerHostConfig.Memory = hc.Memory
		}
		if hc.MemorySwap != 0 && w.InnerHostConfig.MemorySwap == 0 {
			w.InnerHostConfig.MemorySwap = hc.MemorySwap
		}
		if hc.CpuShares != 0 && w.InnerHostConfig.CpuShares == 0 {
			w.InnerHostConfig.CpuShares = hc.CpuShares
		}

		hc = w.InnerHostConfig
	}

	if hc != nil && w.Cpuset != "" && hc.CpusetCpus == "" {
		hc.CpusetCpus = w.Cpuset
	}

	return hc
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

	return w.Config, w.GetHostConfig(), nil
}
