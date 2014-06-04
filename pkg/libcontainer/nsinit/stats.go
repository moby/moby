package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/cgroups/fs"
)

var statsCommand = cli.Command{
	Name:   "stats",
	Usage:  "display statistics for the container",
	Action: statsAction,
}

func statsAction(context *cli.Context) {
	// returns the stats of the current container.
	stats, err := getContainerStats(container)
	if err != nil {
		log.Printf("Failed to get stats - %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Stats:\n%v\n", stats)
	os.Exit(0)
}

// returns the container stats in json format.
func getContainerStats(container *libcontainer.Container) (string, error) {
	stats, err := fs.GetStats(container.Cgroups)
	if err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(stats, "", "\t")
	if err != nil {
		return "", err
	}
	return string(out), nil
}
