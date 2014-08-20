package main

import (
	"os"
	"path/filepath"

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
	flVersion     = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon      = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flDebug       = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flSocketGroup = flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
	flEnableCors  = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
	flTls         = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
	flTlsVerify   = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")

	// these are initialized in init() below since their default values depend on dockerCertPath which isn't fully initialized until init() runs
	flCa    *string
	flCert  *string
	flKey   *string
	flHosts []string
)

func init() {
	flCa = flag.String([]string{"-tlscacert"}, filepath.Join(dockerCertPath, defaultCaFile), "Trust only remotes providing a certificate signed by the CA given here")
	flCert = flag.String([]string{"-tlscert"}, filepath.Join(dockerCertPath, defaultCertFile), "Path to TLS certificate file")
	flKey = flag.String([]string{"-tlskey"}, filepath.Join(dockerCertPath, defaultKeyFile), "Path to TLS key file")
	opts.HostListVar(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode\nspecified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")
}
