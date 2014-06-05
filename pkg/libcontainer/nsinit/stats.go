package main

import (
	"encoding/json"
	"fmt"
	"log"

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
	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	stats, err := getContainerStats(container)
	if err != nil {
		log.Fatalf("Failed to get stats - %v\n", err)
	}

	fmt.Printf("Stats:\n%v\n", stats)
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
