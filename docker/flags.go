package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
)

var (
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTlsVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""
)

func init() {
	if dockerCertPath == "" {
		dockerCertPath = filepath.Join(getHomeDir(), ".docker")
	}
}

func getHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

var (
	flVersion     = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon      = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flDebug       = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flSocketGroup = flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when running in daemon mode\nuse '' (the empty string) to disable setting of a group")
	flLogLevel    = flag.String([]string{"l", "-log-level"}, "info", "Set the logging level")
	flEnableCors  = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
	flTls         = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by --tlsverify flag")
	flTlsVerify   = flag.Bool([]string{"-tlsverify"}, dockerTlsVerify, "Use TLS and verify the remote (daemon: verify client, client: verify daemon)")

	// these are initialized in init() below since their default values depend on dockerCertPath which isn't fully initialized until init() runs
	flTrustKey *string
	flCa       *string
	flCert     *string
	flKey      *string
	flHosts    []string
)

func init() {
	// placeholder for trust key flag
	trustKeyDefault := filepath.Join(dockerCertPath, defaultTrustKeyFile)
	flTrustKey = &trustKeyDefault

	flCa = flag.String([]string{"-tlscacert"}, filepath.Join(dockerCertPath, defaultCaFile), "Trust only remotes providing a certificate signed by the CA given here")
	flCert = flag.String([]string{"-tlscert"}, filepath.Join(dockerCertPath, defaultCertFile), "Path to TLS certificate file")
	flKey = flag.String([]string{"-tlskey"}, filepath.Join(dockerCertPath, defaultKeyFile), "Path to TLS key file")
	opts.HostListVar(&flHosts, []string{"H", "-host"}, "The socket(s) to bind to in daemon mode or connect to in client mode, specified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: docker [OPTIONS] COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nOptions:\n")

		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, command := range [][]string{
			{"attach", "Attach to a running container"},
			{"build", "Build an image from a Dockerfile"},
			{"commit", "Create a new image from a container's changes"},
			{"cp", "Copy files/folders from a container's filesystem to the host path"},
			{"create", "Create a new container"},
			{"diff", "Inspect changes on a container's filesystem"},
			{"events", "Get real time events from the server"},
			{"exec", "Run a command in a running container"},
			{"export", "Stream the contents of a container as a tar archive"},
			{"history", "Show the history of an image"},
			{"images", "List images"},
			{"import", "Create a new filesystem image from the contents of a tarball"},
			{"info", "Display system-wide information"},
			{"inspect", "Return low-level information on a container"},
			{"kill", "Kill a running container"},
			{"load", "Load an image from a tar archive"},
			{"login", "Register or log in to a Docker registry server"},
			{"logout", "Log out from a Docker registry server"},
			{"logs", "Fetch the logs of a container"},
			{"port", "Lookup the public-facing port that is NAT-ed to PRIVATE_PORT"},
			{"pause", "Pause all processes within a container"},
			{"ps", "List containers"},
			{"pull", "Pull an image or a repository from a Docker registry server"},
			{"push", "Push an image or a repository to a Docker registry server"},
			{"restart", "Restart a running container"},
			{"rm", "Remove one or more containers"},
			{"rmi", "Remove one or more images"},
			{"run", "Run a command in a new container"},
			{"save", "Save an image to a tar archive"},
			{"search", "Search for an image on the Docker Hub"},
			{"start", "Start a stopped container"},
			{"stop", "Stop a running container"},
			{"tag", "Tag an image into a repository"},
			{"top", "Lookup the running processes of a container"},
			{"unpause", "Unpause a paused container"},
			{"version", "Show the Docker version information"},
			{"wait", "Block until a container stops, then print its exit code"},
		} {
			help += fmt.Sprintf("    %-10.10s%s\n", command[0], command[1])
		}
		help += "\nRun 'docker COMMAND --help' for more information on a command."
		fmt.Fprintf(os.Stderr, "%s\n", help)
	}
}
