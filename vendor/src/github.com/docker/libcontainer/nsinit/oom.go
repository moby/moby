package main

import (
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
)

var oomCommand = cli.Command{
	Name:   "oom",
	Usage:  "display oom notifications for a container",
	Action: oomAction,
}

func oomAction(context *cli.Context) {
	state, err := libcontainer.GetState(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	n, err := libcontainer.NotifyOnOOM(state)
	if err != nil {
		log.Fatal(err)
	}
	for _ = range n {
		log.Printf("OOM notification received")
	}
}
