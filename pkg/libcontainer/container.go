package libcontainer

type Container struct {
	ID           string       `json:"id,omitempty"`
	NsPid        int          `json:"namespace_pid,omitempty"`
	Command      *Command     `json:"command,omitempty"`
	RootFs       string       `json:"rootfs,omitempty"`
	ReadonlyFs   bool         `json:"readonly_fs,omitempty"`
	NetNsFd      uintptr      `json:"network_namespace_fd,omitempty"`
	User         string       `json:"user,omitempty"`
	WorkingDir   string       `json:"working_dir,omitempty"`
	Namespaces   Namespaces   `json:"namespaces,omitempty"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

type Command struct {
	Args []string `json:"args,omitempty"`
	Env  []string `json:"environment,omitempty"`
}

type Network struct {
	TempVethName string `json:"temp_veth,omitempty"`
	IP           string `json:"ip,omitempty"`
	Gateway      string `json:"gateway,omitempty"`
	Bridge       string `json:"bridge,omitempty"`
	Mtu          int    `json:"mtu,omitempty"`
}
