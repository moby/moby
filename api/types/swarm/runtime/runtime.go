package runtime

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
