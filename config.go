package docker

import (
	"github.com/dotcloud/docker/engine"
	"net"
)

// FIXME: separate runtime configuration from http api configuration
type DaemonConfig struct {
	Pidfile                     string
	Root                        string
	AutoRestart                 bool
	EnableCors                  bool
	Dns                         []string
	EnableIptables              bool
	BridgeIface                 string
	BridgeIp                    string
	DefaultIp                   net.IP
	InterContainerCommunication bool
	GraphDriver                 string
}

// ConfigFromJob creates and returns a new DaemonConfig object
// by parsing the contents of a job's environment.
func ConfigFromJob(job *engine.Job) *DaemonConfig {
	var config DaemonConfig
	config.Pidfile = job.Getenv("Pidfile")
	config.Root = job.Getenv("Root")
	config.AutoRestart = job.GetenvBool("AutoRestart")
	config.EnableCors = job.GetenvBool("EnableCors")
	if dns := job.GetenvList("Dns"); dns != nil {
		config.Dns = dns
	}
	config.EnableIptables = job.GetenvBool("EnableIptables")
	if br := job.Getenv("BridgeIface"); br != "" {
		config.BridgeIface = br
	} else {
		config.BridgeIface = DefaultNetworkBridge
	}
	config.BridgeIp = job.Getenv("BridgeIp")
	config.DefaultIp = net.ParseIP(job.Getenv("DefaultIp"))
	config.InterContainerCommunication = job.GetenvBool("InterContainerCommunication")
	config.GraphDriver = job.Getenv("GraphDriver")
	return &config
}
