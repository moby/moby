// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Plugin A plugin for the Engine API
type Plugin struct {
	// The config of a plugin.
	// Required: true
	Config Config `json:"Config"`

	// True if the plugin is running. False if the plugin is not running, only installed.
	// Example: true
	// Required: true
	Enabled bool `json:"Enabled"`

	//
	// Example: 5724e2c8652da337ab2eedd19fc6fc0ec908e4bd907c7421bf6a8dfc70c4c078
	Id string `json:"Id,omitempty"`

	//
	// Example: tiborvass/sample-volume-plugin
	// Required: true
	Name string `json:"Name"`

	// plugin remote reference used to push/pull the plugin
	// Example: localhost:5000/tiborvass/sample-volume-plugin:latest
	PluginReference string `json:"PluginReference,omitempty"`

	// user-configurable settings for the plugin.
	// Required: true
	Settings Settings `json:"Settings"`
}
