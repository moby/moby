package main

import flag "github.com/docker/docker/pkg/mflag"

var (
	flHelp    = flag.Bool([]string{"h", "-help"}, false, "Print usage")
	flVersion = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
)

type command struct {
	name        string
	description string
}

type byName []command

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].name < a[j].name }

// TODO(tiborvass): do not show 'daemon' on client-only binaries
// and deduplicate description in dockerCommands and cli subcommands
var dockerCommands = []command{
	{"attach", "Attach to a running container"},
	{"build", "Build an image from a Dockerfile"},
	{"commit", "Create a new image from a container's changes"},
	{"cp", "Copy files/folders from a container to a HOSTDIR or to STDOUT"},
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
	{"port", "List port mappings or a specific mapping for the CONTAINER"},
	{"pause", "Pause all processes within a container"},
	{"ps", "List containers"},
	{"pull", "Pull an image or a repository from a registry"},
	{"push", "Push an image or a repository to a registry"},
	{"rename", "Rename a container"},
	{"restart", "Restart a running container"},
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
	{"wait", "Block until a container stops, then print its exit code"},
}
