package cli

import (
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/tlsconfig"
)

// CommonFlags represents flags that are common to both the client and the daemon.
type CommonFlags struct {
	FlagSet   *flag.FlagSet
	PostParse func()

	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	TrustKey   string
}

// Command is the struct contains command name and description
type Command struct {
	Name        string
	Description string
}

var dockerCommands = []Command{
	{"attach", "Attach to a running container"},
	{"build", "Build an image from a Dockerfile"},
	{"commit", "Create a new image from a container's changes"},
	{"cp", "Copy files/folders between a container and the local filesystem"},
	{"create", "Create a new container"},
	{"diff", "Inspect changes on a container's filesystem"},
	{"events", "Get real time events from the server"},
	{"exec", "Run a command in a running container"},
	{"export", "Export a container's filesystem as a tar archive"},
	{"history", "Show the history of an image"},
	{"images", "List images"},
	{"import", "Import the contents from a tarball to create a filesystem image"},
	{"info", "Display system-wide information"},
	{"inspect", "Return low-level information on a container or image"},
	{"kill", "Kill a running container"},
	{"load", "Load an image from a tar archive or STDIN"},
	{"login", "Register or log in to a Docker registry"},
	{"logout", "Log out from a Docker registry"},
	{"logs", "Fetch the logs of a container"},
	{"network", "Manage Docker networks"},
	{"pause", "Pause all processes within a container"},
	{"port", "List port mappings or a specific mapping for the CONTAINER"},
	{"ps", "List containers"},
	{"pull", "Pull an image or a repository from a registry"},
	{"push", "Push an image or a repository to a registry"},
	{"rename", "Rename a container"},
	{"restart", "Restart a container"},
	{"rm", "Remove one or more containers"},
	{"rmi", "Remove one or more images"},
	{"run", "Run a command in a new container"},
	{"save", "Save an image(s) to a tar archive"},
	{"search", "Search the Docker Hub for images"},
	{"start", "Start one or more stopped containers"},
	{"stats", "Display a live stream of container(s) resource usage statistics"},
	{"stop", "Stop a running container"},
	{"tag", "Tag an image into a repository"},
	{"top", "Display the running processes of a container"},
	{"unpause", "Unpause all processes within a container"},
	{"version", "Show the Docker version information"},
	{"volume", "Manage Docker volumes"},
	{"wait", "Block until a container stops, then print its exit code"},
}

// DockerCommands stores all the docker command
var DockerCommands = make(map[string]Command)

func init() {
	for _, cmd := range dockerCommands {
		DockerCommands[cmd.Name] = cmd
	}
}
