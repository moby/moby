// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Device
type Device struct {
	//
	// Required: true
	Description string `json:"Description"`

	//
	// Required: true
	Name string `json:"Name"`

	//
	// Example: /dev/fuse
	Path string `json:"Path,omitempty"`

	//
	// Required: true
	Settable []string `json:"Settable"`
}
