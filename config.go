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
	Dns                         []string
	EnableIptables              bool
	EnableIpForward             bool
	BridgeIface                 string
	BridgeIp                    string
	DefaultIp                   net.IP
	InterContainerCommunication bool
	GraphDriver                 string
	Mtu                         int
}

// ConfigFromJob creates and returns a new DaemonConfig object
// by parsing the contents of a job's environment.
func DaemonConfigFromJob(job *engine.Job) *DaemonConfig {
	config := &DaemonConfig{
		Pidfile:                     job.Getenv("Pidfile"),
		Root:                        job.Getenv("Root"),
		AutoRestart:                 job.GetenvBool("AutoRestart"),
		EnableIptables:              job.GetenvBool("EnableIptables"),
		EnableIpForward:             job.GetenvBool("EnableIpForward"),
		BridgeIp:                    job.Getenv("BridgeIp"),
		DefaultIp:                   net.ParseIP(job.Getenv("DefaultIp")),
		InterContainerCommunication: job.GetenvBool("InterContainerCommunication"),
		GraphDriver:                 job.Getenv("GraphDriver"),
	}
	if dns := job.GetenvList("Dns"); dns != nil {
		config.Dns = dns
	}
	if br := job.Getenv("BridgeIface"); br != "" {
		config.BridgeIface = br
	} else {
		config.BridgeIface = DefaultNetworkBridge
	}
	if mtu := job.GetenvInt("Mtu"); mtu != -1 {
		config.Mtu = mtu
	} else {
		config.Mtu = DefaultNetworkMtu
	}

	return config
}
