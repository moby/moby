// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Env
type Env struct {
	//
	// Required: true
	Description string `json:"Description"`

	//
	// Required: true
	Name string `json:"Name"`

	//
	// Required: true
	Settable []string `json:"Settable"`

	//
	Value string `json:"Value,omitempty"`
}
