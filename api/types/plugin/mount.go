// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Mount
type Mount struct {
	//
	// Example: This is a mount that's used by the plugin.
	// Required: true
	Description string `json:"Description"`

	//
	// Example: /mnt/state
	// Required: true
	Destination string `json:"Destination"`

	//
	// Example: some-mount
	// Required: true
	Name string `json:"Name"`

	//
	// Example: rbind
	// rw
	// Required: true
	Options []string `json:"Options"`

	//
	// Required: true
	Settable []string `json:"Settable"`

	//
	// Example: /var/lib/docker/plugins/
	// Required: true
	Source string `json:"Source"`

	//
	// Example: bind
	// Required: true
	Type string `json:"Type"`
}
