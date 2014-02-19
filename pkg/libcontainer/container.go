package libcontainer

type Container struct {
	ID           string       `json:"id,omitempty"`
	Command      *Command     `json:"command,omitempty"`
	ReadonlyFs   bool         `json:"readonly_fs,omitempty"`
	User         string       `json:"user,omitempty"`
	WorkingDir   string       `json:"working_dir,omitempty"`
	Namespaces   Namespaces   `json:"namespaces,omitempty"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
	LogFile      string       `json:"log_file,omitempty"`
	Network      *Network     `json:"network,omitempty"`
}

type Command struct {
	Args []string `json:"args,omitempty"`
	Env  []string `json:"environment,omitempty"`
}

type Network struct {
	IP           string `json:"ip,omitempty"`
	Gateway      string `json:"gateway,omitempty"`
	Bridge       string `json:"bridge,omitempty"`
	Mtu          int    `json:"mtu,omitempty"`
	TempVethName string `json:"temp_veth,omitempty"`
}
