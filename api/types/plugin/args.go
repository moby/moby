// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Args
type Args struct {
	//
	// Example: args
	// Required: true
	Name string `json:"Name"`

	//
	// Example: command line arguments
	// Required: true
	Description string `json:"Description"`

	//
	// Required: true
	Settable []string `json:"Settable"`

	//
	// Required: true
	Value []string `json:"Value"`
}
