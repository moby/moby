package main

import (
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer/namespaces"
)

var nsenterCommand = cli.Command{
	Name:   "nsenter",
	Usage:  "init process for entering an existing namespace",
	Action: nsenterAction,
	Flags: []cli.Flag{
		cli.IntFlag{Name: "nspid"},
		cli.StringFlag{Name: "containerjson"},
	},
}

func nsenterAction(context *cli.Context) {
	args := context.Args()

	if len(args) == 0 {
		args = []string{"/bin/bash"}
	}

	container, err := loadContainerFromJson(context.String("containerjson"))
	if err != nil {
		log.Fatalf("unable to load container: %s", err)
	}

	nspid := context.Int("nspid")
	if nspid <= 0 {
		log.Fatalf("cannot enter into namespaces without valid pid: %q", nspid)
	}

	if err := namespaces.NsEnter(container, nspid, args); err != nil {
		log.Fatalf("failed to nsenter: %s", err)
	}
}
