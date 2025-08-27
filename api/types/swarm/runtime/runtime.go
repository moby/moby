package runtime

import "fmt"

// PluginSpec defines the base payload which clients can specify for creating
// a service with the plugin runtime.
type PluginSpec struct {
	Name       string             `json:"name,omitempty"`
	Remote     string             `json:"remote,omitempty"`
	Privileges []*PluginPrivilege `json:"privileges,omitempty"`
	Disabled   bool               `json:"disabled,omitempty"`
	Env        []string           `json:"env,omitempty"`
}

// PluginPrivilege describes a permission the user has to accept
// upon installing a plugin.
type PluginPrivilege struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Value       []string `json:"value,omitempty"`
}

var (
	ErrInvalidLengthPlugin        = fmt.Errorf("proto: negative length found during unmarshaling") // Deprecated: this error was only used internally and is no longer used.
	ErrIntOverflowPlugin          = fmt.Errorf("proto: integer overflow")                          // Deprecated: this error was only used internally and is no longer used.
	ErrUnexpectedEndOfGroupPlugin = fmt.Errorf("proto: unexpected end of group")                   // Deprecated: this error was only used internally and is no longer used.
)
