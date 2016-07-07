package cli

// Command is the struct containing the command name and description
type Command struct {
	Name        string
	Description string
}

// DockerCommandUsage lists the top level docker commands and their short usage
var DockerCommandUsage = []Command{
	{"exec", "Run a command in a running container"},
	{"inspect", "Return low-level information on a container, image or task"},
}

// DockerCommands stores all the docker command
var DockerCommands = make(map[string]Command)

func init() {
	for _, cmd := range DockerCommandUsage {
		DockerCommands[cmd.Name] = cmd
	}
}
