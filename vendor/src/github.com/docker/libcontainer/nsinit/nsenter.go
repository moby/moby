package nsinit

import (
	"log"
	"strconv"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer/namespaces"
)

var nsenterCommand = cli.Command{
	Name:   "nsenter",
	Usage:  "init process for entering an existing namespace",
	Action: nsenterAction,
}

func nsenterAction(context *cli.Context) {
	args := context.Args()

	if len(args) == 0 {
		args = []string{"/bin/bash"}
	}

	container, err := loadContainerFromJson(context.GlobalString("containerjson"))
	if err != nil {
		log.Fatalf("unable to load container: %s", err)
	}

	nspid, err := strconv.Atoi(context.GlobalString("nspid"))
	if nspid <= 0 || err != nil {
		log.Fatalf("cannot enter into namespaces without valid pid: %q - %s", nspid, err)
	}

	if err := namespaces.NsEnter(container, args); err != nil {
		log.Fatalf("failed to nsenter: %s", err)
	}
}
