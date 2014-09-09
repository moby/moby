// +build linux

package native

import (
	"os"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/syncpipe"
)

func findUserArgs() []string {
	i := 0
	for _, a := range os.Args {
		i++

		if a == "--" {
			break
		}
	}

	return os.Args[i:]
}

// loadConfigFromFd loads a container's config from the sync pipe that is provided by
// fd 3 when running a process
func loadConfigFromFd() (*libcontainer.Config, error) {
	syncPipe, err := syncpipe.NewSyncPipeFromFd(0, 3)
	if err != nil {
		return nil, err
	}

	var config *libcontainer.Config
	if err := syncPipe.ReadFromParent(&config); err != nil {
		return nil, err
	}

	return config, nil
}
