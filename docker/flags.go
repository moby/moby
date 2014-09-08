package main

import (
	"fmt"
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
	flVersion     = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon      = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flDebug       = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flSocketGroup = flag.String([]string{"G", "-group"}, "docker", "Group to assign the unix socket specified by -H when\nrunning in daemon mode. Use \"\" (the empty string) to\ndisable setting a group. (default: \"docker\")")
	flEnableCors  = flag.Bool([]string{"#api-enable-cors", "-api-enable-cors"}, false, "Enable CORS headers in the remote API")
	flTls         = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by tls-verify flags")
	flTlsVerify   = flag.Bool([]string{"-tlsverify"}, false, "Use TLS and verify the remote\n(daemon: verify client, client: verify daemon)")

	// these are initialized in init() below since their default values depend on dockerCertPath which isn't fully initialized until init() runs
	flCa    *string
	flCert  *string
	flKey   *string
	flHosts []string
)

func init() {
	defaultCaFilePath := filepath.Join(dockerCertPath, defaultCaFile)
	flCa = flag.String([]string{"-tlscacert"}, defaultCaFilePath, fmt.Sprintf("Only trust remotes providing certificate signed by CA\n(default: %q)", defaultCaFilePath))
	defaultCertFilePath := filepath.Join(dockerCertPath, defaultCertFile)
	flCert = flag.String([]string{"-tlscert"}, defaultCaFilePath, fmt.Sprintf("Path to TLS certificate file\n(default: %q)", defaultCertFilePath))
	defaultKeyFilePath := filepath.Join(dockerCertPath, defaultKeyFile)
	flKey = flag.String([]string{"-tlskey"}, defaultKeyFilePath, fmt.Sprintf("Path to TLS key file\n(default: %q)", defaultKeyFilePath))
	opts.HostListVar(&flHosts, []string{"H", "-host"}, fmt.Sprintf("The socket(s) to bind to in daemon mode or connect to in\nclient mode. Specified using one or more tcp://host:port,\nunix:///path/to/socket, fd://* or fd://socketfd.\n(default: %q)", api.DEFAULTUNIXSOCKET))

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: docker [OPTIONS] COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nOptions:\n")

		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, command := range [][]string{
			{"attach", "Attach to a running container"},
			{"build", "Build an image from a Dockerfile"},
			{"commit", "Create a new image from a container's changes"},
			{"cp", "Copy files/folders from a container's filesystem to the host path"},
			{"diff", "Inspect changes on a container's filesystem"},
			{"events", "Get real time events from the server"},
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
