// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// Interface The interface between Docker and the plugin
type Interface struct {
	//
	// Example: docker.volumedriver/1.0
	// Required: true
	Types []CapabilityID `json:"Types"`

	//
	// Example: plugins.sock
	// Required: true
	Socket string `json:"Socket"`

	// Protocol to use for clients connecting to the plugin.
	// Example: some.protocol/v1.0
	// Enum: : [, moby.plugins.http/v1]
	ProtocolScheme string `json:"ProtocolScheme,omitempty"`
}
