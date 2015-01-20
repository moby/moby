// +build linux

package native

import (
	"encoding/json"
	"os"

	"github.com/docker/libcontainer"
)

func findUserArgs() []string {
	for i, a := range os.Args {
		if a == "--" {
			return os.Args[i+1:]
		}
	}
	return []string{}
}

// loadConfigFromFd loads a container's config from the sync pipe that is provided by
// fd 3 when running a process
func loadConfigFromFd() (*libcontainer.Config, error) {
	var config *libcontainer.Config
	if err := json.NewDecoder(os.NewFile(3, "child")).Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}
