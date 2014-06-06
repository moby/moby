package main

import (
	"log"
	"strconv"

	"github.com/codegangsta/cli"
	"github.com/dotcloud/docker/pkg/libcontainer/namespaces"
)

var nsenterCommand = cli.Command{
	Name:   "nsenter",
	Usage:  "init process for entering an existing namespace",
	Action: nsenterAction,
}

func nsenterAction(context *cli.Context) {
	args := context.Args()
	if len(args) < 4 {
		log.Fatalf("incorrect usage: <pid> <process label> <container JSON> <cmd>...")
	}

	container, err := loadContainerFromJson(args.Get(2))
	if err != nil {
		log.Fatalf("unable to load container: %s", err)
	}

	nspid, err := strconv.Atoi(args.Get(0))
	if err != nil {
		log.Fatalf("unable to read pid: %s from %q", err, args.Get(0))
	}

	if nspid <= 0 {
		log.Fatalf("cannot enter into namespaces without valid pid: %q", nspid)
	}

	if err := namespaces.NsEnter(container, args.Get(1), nspid, args[3:]); err != nil {
		log.Fatalf("failed to nsenter: %s", err)
	}
}
