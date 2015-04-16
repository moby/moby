package runconfig

import (
	"encoding/json"
	"io"

	"github.com/docker/docker/nat"
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
		p = append(p, string(b))
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
		p = append(p, string(b))
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
type Config struct {
	Hostname        string
	Domainname      string
	User            string
	AttachStdin     bool
	AttachStdout    bool
	AttachStderr    bool
	PortSpecs       []string // Deprecated - Can be in the format of 8080/tcp
	ExposedPorts    map[nat.Port]struct{}
	Tty             bool // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin       bool // Open stdin
	StdinOnce       bool // If true, close stdin after the 1 attached client disconnects.
	Env             []string
	Cmd             *Command
	Image           string // Name of the image as it was passed by the operator (eg. could be symbolic)
	Volumes         map[string]struct{}
	WorkingDir      string
	Entrypoint      *Entrypoint
	NetworkDisabled bool
	MacAddress      string
	OnBuild         []string
	Labels          map[string]string
}

type ContainerConfigWrapper struct {
	*Config
	*hostConfigWrapper
}

func (c ContainerConfigWrapper) HostConfig() *HostConfig {
	if c.hostConfigWrapper == nil {
		return new(HostConfig)
	}

	return c.hostConfigWrapper.GetHostConfig()
}

func DecodeContainerConfig(src io.Reader) (*Config, *HostConfig, error) {
	decoder := json.NewDecoder(src)

	var w ContainerConfigWrapper
	if err := decoder.Decode(&w); err != nil {
		return nil, nil, err
	}

	return w.Config, w.HostConfig(), nil
}
