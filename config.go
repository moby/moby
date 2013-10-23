package docker

import (
	"net"
	"github.com/dotcloud/docker/engine"
)

// FIXME: separate runtime configuration from http api configuration
type DaemonConfig struct {
	Pidfile                     string
	Root                        string
	ProtoAddresses              []string
	AutoRestart                 bool
	EnableCors                  bool
	Dns                         []string
	EnableIptables              bool
	BridgeIface                 string
	DefaultIp                   net.IP
	InterContainerCommunication bool
}

// ConfigGetenv creates and returns a new DaemonConfig object
// by parsing the contents of a job's environment.
func ConfigGetenv(job *engine.Job) *DaemonConfig {
	var config DaemonConfig
	config.Pidfile = job.Getenv("Pidfile")
	config.Root = job.Getenv("Root")
	config.AutoRestart = job.GetenvBool("AutoRestart")
	config.EnableCors = job.GetenvBool("EnableCors")
	if dns := job.Getenv("Dns"); dns != "" {
		config.Dns = []string{dns}
	}
	config.EnableIptables = job.GetenvBool("EnableIptables")
	if br := job.Getenv("BridgeIface"); br != "" {
		config.BridgeIface = br
	} else {
		config.BridgeIface = DefaultNetworkBridge
	}
	config.ProtoAddresses = job.GetenvList("ProtoAddresses")
	config.DefaultIp = net.ParseIP(job.Getenv("DefaultIp"))
	config.InterContainerCommunication = job.GetenvBool("InterContainerCommunication")
	return &config
}
