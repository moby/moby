package plugins // import "github.com/docker/docker/api/types/plugins"

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Info is a temp struct holding Plugins name
// registered with docker daemon. It is used by Info struct
type Info struct {
	// List of Volume plugins registered
	Volume []string
	// List of Network plugins registered
	Network []string
	// List of Authorization plugins registered
	Authorization []string
	// List of Log plugins registered
	Log []string
}

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
type PluginPrivilege struct {
	Name        string
	Description string
	Value       []string
}

// PluginPrivileges is a list of PluginPrivilege
type PluginPrivileges []PluginPrivilege

func (s PluginPrivileges) Len() int {
	return len(s)
}

func (s PluginPrivileges) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s PluginPrivileges) Swap(i, j int) {
	sort.Strings(s[i].Value)
	sort.Strings(s[j].Value)
	s[i], s[j] = s[j], s[i]
}

// Plugin A plugin for the Engine API
// swagger:model Plugin
type Plugin struct {

	// config
	// Required: true
	Config PluginConfig `json:"Config"`

	// True if the plugin is running. False if the plugin is not running, only installed.
	// Required: true
	Enabled bool `json:"Enabled"`

	// Id
	ID string `json:"Id,omitempty"`

	// name
	// Required: true
	Name string `json:"Name"`

	// plugin remote reference used to push/pull the plugin
	PluginReference string `json:"PluginReference,omitempty"`

	// settings
	// Required: true
	Settings PluginSettings `json:"Settings"`
}

// PluginConfig The config of a plugin.
// swagger:model PluginConfig
type PluginConfig struct {

	// args
	// Required: true
	Args PluginConfigArgs `json:"Args"`

	// description
	// Required: true
	Description string `json:"Description"`

	// Docker Version used to create the plugin
	DockerVersion string `json:"DockerVersion,omitempty"`

	// documentation
	// Required: true
	Documentation string `json:"Documentation"`

	// entrypoint
	// Required: true
	Entrypoint []string `json:"Entrypoint"`

	// env
	// Required: true
	Env []PluginEnv `json:"Env"`

	// interface
	// Required: true
	Interface PluginConfigInterface `json:"Interface"`

	// ipc host
	// Required: true
	IpcHost bool `json:"IpcHost"`

	// linux
	// Required: true
	Linux PluginConfigLinux `json:"Linux"`

	// mounts
	// Required: true
	Mounts []PluginMount `json:"Mounts"`

	// network
	// Required: true
	Network PluginConfigNetwork `json:"Network"`

	// pid host
	// Required: true
	PidHost bool `json:"PidHost"`

	// propagated mount
	// Required: true
	PropagatedMount string `json:"PropagatedMount"`

	// user
	User PluginConfigUser `json:"User,omitempty"`

	// work dir
	// Required: true
	WorkDir string `json:"WorkDir"`

	// rootfs
	Rootfs *PluginConfigRootfs `json:"rootfs,omitempty"`
}

// PluginConfigArgs plugin config args
// swagger:model PluginConfigArgs
type PluginConfigArgs struct {

	// description
	// Required: true
	Description string `json:"Description"`

	// name
	// Required: true
	Name string `json:"Name"`

	// settable
	// Required: true
	Settable []string `json:"Settable"`

	// value
	// Required: true
	Value []string `json:"Value"`
}

// PluginConfigInterface The interface between Docker and the plugin
// swagger:model PluginConfigInterface
type PluginConfigInterface struct {

	// socket
	// Required: true
	Socket string `json:"Socket"`

	// types
	// Required: true
	Types []PluginInterfaceType `json:"Types"`
}

// PluginConfigLinux plugin config linux
// swagger:model PluginConfigLinux
type PluginConfigLinux struct {

	// allow all devices
	// Required: true
	AllowAllDevices bool `json:"AllowAllDevices"`

	// capabilities
	// Required: true
	Capabilities []string `json:"Capabilities"`

	// devices
	// Required: true
	Devices []PluginDevice `json:"Devices"`
}

// PluginConfigNetwork plugin config network
// swagger:model PluginConfigNetwork
type PluginConfigNetwork struct {

	// type
	// Required: true
	Type string `json:"Type"`
}

// PluginConfigRootfs plugin config rootfs
// swagger:model PluginConfigRootfs
type PluginConfigRootfs struct {

	// diff ids
	DiffIds []string `json:"diff_ids"`

	// type
	Type string `json:"type,omitempty"`
}

// PluginConfigUser plugin config user
// swagger:model PluginConfigUser
type PluginConfigUser struct {

	// g ID
	GID uint32 `json:"GID,omitempty"`

	// UID
	UID uint32 `json:"UID,omitempty"`
}

// PluginSettings Settings that can be modified by users.
// swagger:model PluginSettings
type PluginSettings struct {

	// args
	// Required: true
	Args []string `json:"Args"`

	// devices
	// Required: true
	Devices []PluginDevice `json:"Devices"`

	// env
	// Required: true
	Env []string `json:"Env"`

	// mounts
	// Required: true
	Mounts []PluginMount `json:"Mounts"`
}

// PluginDevice plugin device
// swagger:model PluginDevice
type PluginDevice struct {

	// description
	// Required: true
	Description string `json:"Description"`

	// name
	// Required: true
	Name string `json:"Name"`

	// path
	// Required: true
	Path *string `json:"Path"`

	// settable
	// Required: true
	Settable []string `json:"Settable"`
}

// PluginEnv plugin env
// swagger:model PluginEnv
type PluginEnv struct {

	// description
	// Required: true
	Description string `json:"Description"`

	// name
	// Required: true
	Name string `json:"Name"`

	// settable
	// Required: true
	Settable []string `json:"Settable"`

	// value
	// Required: true
	Value *string `json:"Value"`
}

// PluginInterfaceType plugin interface type
// swagger:model PluginInterfaceType
type PluginInterfaceType struct {

	// capability
	// Required: true
	Capability string `json:"Capability"`

	// prefix
	// Required: true
	Prefix string `json:"Prefix"`

	// version
	// Required: true
	Version string `json:"Version"`
}

// PluginMount plugin mount
// swagger:model PluginMount
type PluginMount struct {

	// description
	// Required: true
	Description string `json:"Description"`

	// destination
	// Required: true
	Destination string `json:"Destination"`

	// name
	// Required: true
	Name string `json:"Name"`

	// options
	// Required: true
	Options []string `json:"Options"`

	// settable
	// Required: true
	Settable []string `json:"Settable"`

	// source
	// Required: true
	Source *string `json:"Source"`

	// type
	// Required: true
	Type string `json:"Type"`
}

// PluginsListResponse contains the response for the Engine API
type PluginsListResponse []*Plugin

// UnmarshalJSON implements json.Unmarshaler for PluginInterfaceType
func (t *PluginInterfaceType) UnmarshalJSON(p []byte) error {
	versionIndex := len(p)
	prefixIndex := 0
	if len(p) < 2 || p[0] != '"' || p[len(p)-1] != '"' {
		return fmt.Errorf("%q is not a plugin interface type", p)
	}
	p = p[1 : len(p)-1]
loop:
	for i, b := range p {
		switch b {
		case '.':
			prefixIndex = i
		case '/':
			versionIndex = i
			break loop
		}
	}
	t.Prefix = string(p[:prefixIndex])
	t.Capability = string(p[prefixIndex+1 : versionIndex])
	if versionIndex < len(p) {
		t.Version = string(p[versionIndex+1:])
	}
	return nil
}

// MarshalJSON implements json.Marshaler for PluginInterfaceType
func (t *PluginInterfaceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// String implements fmt.Stringer for PluginInterfaceType
func (t PluginInterfaceType) String() string {
	return fmt.Sprintf("%s.%s/%s", t.Prefix, t.Capability, t.Version)
}
