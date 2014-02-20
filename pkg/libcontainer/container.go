package libcontainer

type Container struct {
	Hostname     string       `json:"hostname,omitempty"`
	ReadonlyFs   bool         `json:"readonly_fs,omitempty"`
	User         string       `json:"user,omitempty"`
	WorkingDir   string       `json:"working_dir,omitempty"`
	Env          []string     `json:"environment,omitempty"`
	Namespaces   Namespaces   `json:"namespaces,omitempty"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
	Network      *Network     `json:"network,omitempty"`
}

type Network struct {
	IP      string `json:"ip,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	Bridge  string `json:"bridge,omitempty"`
	Mtu     int    `json:"mtu,omitempty"`
}
