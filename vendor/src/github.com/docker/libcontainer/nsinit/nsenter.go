package nsinit

import (
	"log"
	"os"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/namespaces"
	_ "github.com/docker/libcontainer/namespaces/nsenter"
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

// this expects that we already have our namespaces setup by the C initializer
// we are expected to finalize the namespace and exec the user's application
func nsenter() {
	syncPipe, err := syncpipe.NewSyncPipeFromFd(0, 3)
	if err != nil {
		log.Fatalf("unable to create sync pipe: %s", err)
	}

	var config *libcontainer.Config
	if err := syncPipe.ReadFromParent(&config); err != nil {
		log.Fatalf("reading container config from parent: %s", err)
	}

	if err := namespaces.FinalizeSetns(config, findUserArgs()); err != nil {
		log.Fatalf("failed to nsenter: %s", err)
	}
}
