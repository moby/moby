package main

const daemonBinary = "dockerd"

// DaemonProxy acts as a cli.Handler to proxy calls to the daemon binary
type DaemonProxy struct{}

// NewDaemonProxy returns a new handler
func NewDaemonProxy() DaemonProxy {
	return DaemonProxy{}
}
