// Code generated from OpenAPI definition. DO NOT EDIT.

package plugin

// LinuxConfig
type LinuxConfig struct {
	//
	// Example: CAP_SYS_ADMIN
	// CAP_SYSLOG
	// Required: true
	Capabilities []string `json:"Capabilities"`

	//
	// Example: false
	// Required: true
	AllowAllDevices bool `json:"AllowAllDevices"`

	//
	// Required: true
	Devices []Device `json:"Devices"`
}
