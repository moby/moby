package main

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/api"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
)

var (
	dockerCertPath = os.Getenv("DOCKER_CERT_PATH")
)

func init() {
	if dockerCertPath == "" {
		dockerCertPath = filepath.Join(os.Getenv("HOME"), ".docker")
	}
}

var (
	flVersion            = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon             = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flGraphOpts          opts.ListOpts
	flDebug              = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flAutoRestart        = flag.Bool([]string{"r", "-restart"}, true, "Restart previously running containers")
	bridgeName           = flag.String([]string{"b", "-bridge"}, "", "Attach containers to a pre-existing network bridge\nuse 'none' to disable container networking")
	bridgeIp             = flag.String([]string{"#bip", "-bip"}, "", "Use this CIDR notation address for the network bridge's IP, not compatible with -b")
	pidfile              = flag.String([]string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
	flRoot               = flag.String([]string{"g", "-graph"}, "/var/lib/docker", "Path to use as the root of the Docker runtime")
	flSocketGroup        = flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
	flEnableCors         = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
	flDns                = opts.NewListOpts(opts.ValidateIPAddress)
	flDnsSearch          = opts.NewListOpts(opts.ValidateDnsSearch)
	flEnableIptables     = flag.Bool([]string{"#iptables", "-iptables"}, true, "Enable Docker's addition of iptables rules")
	flEnableIpForward    = flag.Bool([]string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	flDefaultIp          = flag.String([]string{"#ip", "-ip"}, "0.0.0.0", "Default IP address to use when binding container ports")
	flInterContainerComm = flag.Bool([]string{"#icc", "-icc"}, true, "Enable inter-container communication")
	flGraphDriver        = flag.String([]string{"s", "-storage-driver"}, "", "Force the Docker runtime to use a specific storage driver")
	flExecDriver         = flag.String([]string{"e", "-exec-driver"}, "native", "Force the Docker runtime to use a specific exec driver")
	flHosts              = opts.NewListOpts(api.ValidateHost)
	flMtu                = flag.Int([]string{"#mtu", "-mtu"}, 0, "Set the containers network MTU\nif no value is provided: default to the default route MTU or 1500 if no default route is available")
	flTls                = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
	flTlsVerify          = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")
	flSelinuxEnabled     = flag.Bool([]string{"-selinux-enabled"}, false, "Enable selinux support. SELinux does not presently support the BTRFS storage driver")

	// these are initialized in init() below since their default values depend on dockerCertPath which isn't fully initialized until init() runs
	flCa   *string
	flCert *string
	flKey  *string
)

func init() {
	flCa = flag.String([]string{"-tlscacert"}, filepath.Join(dockerCertPath, defaultCaFile), "Trust only remotes providing a certificate signed by the CA given here")
	flCert = flag.String([]string{"-tlscert"}, filepath.Join(dockerCertPath, defaultCertFile), "Path to TLS certificate file")
	flKey = flag.String([]string{"-tlskey"}, filepath.Join(dockerCertPath, defaultKeyFile), "Path to TLS key file")

	flag.Var(&flDns, []string{"#dns", "-dns"}, "Force Docker to use specific DNS servers")
	flag.Var(&flDnsSearch, []string{"-dns-search"}, "Force Docker to use specific DNS search domains")
	flag.Var(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")
	flag.Var(&flGraphOpts, []string{"-storage-opt"}, "Set storage driver options")
}
