// +build linux

package runconfig

import (
	"encoding/json"
	"strings"

	"github.com/docker/docker/pkg/ulimit"
)

type IpcMode string

// IsPrivate indicates whether container use it's private ipc stack
func (n IpcMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

func (n IpcMode) IsHost() bool {
	return n == "host"
}

func (n IpcMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

func (n IpcMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	case "container":
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

func (n IpcMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

type PidMode string

// IsPrivate indicates whether container use it's private pid stack
func (n PidMode) IsPrivate() bool {
	return !(n.IsHost())
}

func (n PidMode) IsHost() bool {
	return n == "host"
}

func (n PidMode) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	default:
		return false
	}
	return true
}

type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

type LxcConfig struct {
	values []KeyValuePair
}

func (c *LxcConfig) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte{}, nil
	}
	return json.Marshal(c.Slice())
}

func (c *LxcConfig) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var kv []KeyValuePair
	if err := json.Unmarshal(b, &kv); err != nil {
		var h map[string]string
		if err := json.Unmarshal(b, &h); err != nil {
			return err
		}
		for k, v := range h {
			kv = append(kv, KeyValuePair{k, v})
		}
	}
	c.values = kv

	return nil
}

func (c *LxcConfig) Len() int {
	if c == nil {
		return 0
	}
	return len(c.values)
}

func (c *LxcConfig) Slice() []KeyValuePair {
	if c == nil {
		return nil
	}
	return c.values
}

func NewLxcConfig(values []KeyValuePair) *LxcConfig {
	return &LxcConfig{values}
}

type HostConfig struct {
	CommonHostConfig

	// Fields below here are platform specific.
	LxcConf        *LxcConfig
	BlkioWeight    int64 // Block IO weight (relative weight vs. other containers)
	OomKillDisable bool  // Whether to disable OOM Killer or not
	Privileged     bool
	ExtraHosts     []string
	VolumesFrom    []string
	Devices        []DeviceMapping
	IpcMode        IpcMode
	CapAdd         []string
	CapDrop        []string
	SecurityOpt    []string
	ReadonlyRootfs bool
	PidMode        PidMode
	Ulimits        []*ulimit.Ulimit
	CgroupParent   string // Parent cgroup.
}
