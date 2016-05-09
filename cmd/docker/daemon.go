package main

const daemonBinary = "dockerd"

// DaemonProxy acts as a cli.Handler to proxy calls to the daemon binary
type DaemonProxy struct{}

// NewDaemonProxy returns a new handler
func NewDaemonProxy() DaemonProxy {
	return DaemonProxy{}
}

// Command returns a cli command handler if one exists
func (p DaemonProxy) Command(name string) func(...string) error {
	return map[string]func(...string) error{
		"daemon": p.CmdDaemon,
	}[name]
}
