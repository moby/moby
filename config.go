package docker

import (
	"net"

	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/networkdriver"
)

const (
	defaultNetworkMtu    = 1500
	DisableNetworkBridge = "none"
)

// FIXME: separate runtime configuration from http api configuration
type DaemonConfig struct {
	Pidfile                     string
	Root                        string
	AutoRestart                 bool
	Dns                         []string
	EnableIptables              bool
	EnableIpForward             bool
	DefaultIp                   net.IP
	BridgeIface                 string
	BridgeIP                    string
	InterContainerCommunication bool
	GraphDriver                 string
	Mtu                         int
	DisableNetwork              bool
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
		BridgeIP:                    job.Getenv("BridgeIP"),
		DefaultIp:                   net.ParseIP(job.Getenv("DefaultIp")),
		InterContainerCommunication: job.GetenvBool("InterContainerCommunication"),
		GraphDriver:                 job.Getenv("GraphDriver"),
	}
	if dns := job.GetenvList("Dns"); dns != nil {
		config.Dns = dns
	}
	if mtu := job.GetenvInt("Mtu"); mtu != 0 {
		config.Mtu = mtu
	} else {
		config.Mtu = GetDefaultNetworkMtu()
	}
	config.DisableNetwork = job.Getenv("BridgeIface") == DisableNetworkBridge

	return config
}

func GetDefaultNetworkMtu() int {
	if iface, err := networkdriver.GetDefaultRouteIface(); err == nil {
		return iface.MTU
	}
	return defaultNetworkMtu
}
