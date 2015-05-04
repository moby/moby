package daemon

import (
	"github.com/docker/docker/daemon/networkdriver"
	"github.com/docker/docker/daemon/networkdriver/bridge"
	"github.com/docker/docker/pkg/ulimit"
	"github.com/docker/docker/runconfig"
)

const (
	defaultNetworkMtu    = 1500
	disableNetworkBridge = "none"
)

// Config define the configuration of a docker daemon
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
// FIXME: separate runtime configuration from http api configuration
type Config struct {
	Bridge bridge.Config

	Pidfile              string
	Root                 string
	AutoRestart          bool
	Dns                  []string
	DnsSearch            []string
	GraphDriver          string
	GraphOptions         []string
	ExecDriver           string
	ExecOptions          []string
	Mtu                  int
	SocketGroup          string
	EnableCors           bool
	CorsHeaders          string
	DisableNetwork       bool
	EnableSelinuxSupport bool
	Context              map[string][]string
	TrustKeyPath         string
	Labels               []string
	Ulimits              map[string]*ulimit.Ulimit
	LogConfig            runconfig.LogConfig
}

func getDefaultNetworkMtu() int {
	if iface, err := networkdriver.GetDefaultRouteIface(); err == nil {
		return iface.MTU
	}
	return defaultNetworkMtu
}
