// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Config The config of a plugin.
type Config struct {
	//
	// Example: A sample volume plugin for Docker
	// Required: true
	Description string `json:"Description"`

	//
	// Example: https://docs.docker.com/engine/extend/plugins/
	// Required: true
	Documentation string `json:"Documentation"`

	// The interface between Docker and the plugin
	// Required: true
	Interface Interface `json:"Interface"`

	//
	// Example: /usr/bin/sample-volume-plugin
	// /data
	// Required: true
	Entrypoint []string `json:"Entrypoint"`

	//
	// Example: /bin/
	// Required: true
	WorkDir string `json:"WorkDir"`

	//
	User *User `json:"User,omitempty"`

	//
	// Required: true
	Network NetworkConfig `json:"Network"`

	//
	// Required: true
	Linux LinuxConfig `json:"Linux"`

	//
	// Example: /mnt/volumes
	// Required: true
	PropagatedMount string `json:"PropagatedMount"`

	//
	// Example: false
	// Required: true
	IpcHost bool `json:"IpcHost"`

	//
	// Example: false
	// Required: true
	PidHost bool `json:"PidHost"`

	//
	// Required: true
	Mounts []Mount `json:"Mounts"`

	//
	// Example: {
	//   "Description": "If set, prints debug messages",
	//   "Name": "DEBUG",
	//   "Settable": null,
	//   "Value": "0"
	// }
	// Required: true
	Env []Env `json:"Env"`

	//
	// Required: true
	Args Args `json:"Args"`

	//
	Rootfs *RootFS `json:"rootfs,omitempty"`
}
