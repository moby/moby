package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/homedir"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/tlsconfig"
)

type command struct {
	name        string
	description string
}

type byName []command

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].name < a[j].name }

var (
	dockerCertPath  = os.Getenv("DOCKER_CERT_PATH")
	dockerTlsVerify = os.Getenv("DOCKER_TLS_VERIFY") != ""

	dockerCommands = []command{
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
		{"inspect", "Return low-level information on a container or image"},
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
		{"rename", "Rename an existing container"},
		{"restart", "Restart a running container"},
		{"rm", "Remove one or more containers"},
		{"rmi", "Remove one or more images"},
		{"run", "Run a command in a new container"},
		{"save", "Save an image to a tar archive"},
		{"search", "Search for an image on the Docker Hub"},
		{"start", "Start a stopped container"},
		{"stats", "Display a stream of a containers' resource usage statistics"},
		{"stop", "Stop a running container"},
		{"tag", "Tag an image into a repository"},
		{"top", "Lookup the running processes of a container"},
		{"unpause", "Unpause a paused container"},
		{"version", "Show the Docker version information"},
		{"wait", "Block until a container stops, then print its exit code"},
	}
)

func init() {
	if dockerCertPath == "" {
		dockerCertPath = filepath.Join(homedir.Get(), ".docker")
	}
}

func getDaemonConfDir() string {
	// TODO: update for Windows daemon
	if runtime.GOOS == "windows" {
		return filepath.Join(homedir.Get(), ".docker")
	}
	return "/etc/docker"
}

var (
	flVersion   = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
	flDaemon    = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flDebug     = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flLogLevel  = flag.String([]string{"l", "-log-level"}, "info", "Set the logging level")
	flTls       = flag.Bool([]string{"-tls"}, false, "Use TLS; implied by --tlsverify")
	flHelp      = flag.Bool([]string{"h", "-help"}, false, "Print usage")
	flTlsVerify = flag.Bool([]string{"-tlsverify"}, dockerTlsVerify, "Use TLS and verify the remote")

	// these are initialized in init() below since their default values depend on dockerCertPath which isn't fully initialized until init() runs
	tlsOptions tlsconfig.Options
	flTrustKey *string
	flHosts    []string
)

func setDefaultConfFlag(flag *string, def string) {
	if *flag == "" {
		if *flDaemon {
			*flag = filepath.Join(getDaemonConfDir(), def)
		} else {
			*flag = filepath.Join(homedir.Get(), ".docker", def)
		}
	}
}

func init() {
	var placeholderTrustKey string
	// TODO use flag flag.String([]string{"i", "-identity"}, "", "Path to libtrust key file")
	flTrustKey = &placeholderTrustKey

	flag.StringVar(&tlsOptions.CAFile, []string{"-tlscacert"}, filepath.Join(dockerCertPath, defaultCaFile), "Trust certs signed only by this CA")
	flag.StringVar(&tlsOptions.CertFile, []string{"-tlscert"}, filepath.Join(dockerCertPath, defaultCertFile), "Path to TLS certificate file")
	flag.StringVar(&tlsOptions.KeyFile, []string{"-tlskey"}, filepath.Join(dockerCertPath, defaultKeyFile), "Path to TLS key file")
	opts.HostListVar(&flHosts, []string{"H", "-host"}, "Daemon socket(s) to connect to")

	flag.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: docker [OPTIONS] COMMAND [arg...]\n\nA self-sufficient runtime for linux containers.\n\nOptions:\n")

		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()

		help := "\nCommands:\n"

		sort.Sort(byName(dockerCommands))

		for _, cmd := range dockerCommands {
			help += fmt.Sprintf("    %-10.10s%s\n", cmd.name, cmd.description)
		}

		help += "\nRun 'docker COMMAND --help' for more information on a command."
		fmt.Fprintf(os.Stdout, "%s\n", help)
	}
}
