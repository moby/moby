// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Settings user-configurable settings for the plugin.
type Settings struct {
	//
	// Required: true
	Mounts []Mount `json:"Mounts"`

	//
	// Example: DEBUG=0
	// Required: true
	Env []string `json:"Env"`

	//
	// Required: true
	Args []string `json:"Args"`

	//
	// Required: true
	Devices []Device `json:"Devices"`
}
