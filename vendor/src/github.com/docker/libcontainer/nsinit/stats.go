package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
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

	runtimeCkpt, err := libcontainer.GetState(dataPath)
	if err != nil {
		log.Fatal(err)
	}

	stats, err := getStats(container, runtimeCkpt)
	if err != nil {
		log.Fatalf("Failed to get stats - %v\n", err)
	}

	fmt.Printf("Stats:\n%v\n", stats)
}

// returns the container stats in json format.
func getStats(container *libcontainer.Config, state *libcontainer.State) (string, error) {
	stats, err := libcontainer.GetStats(container, state)
	if err != nil {
		return "", err
	}

	out, err := json.MarshalIndent(stats, "", "\t")
	if err != nil {
		return "", err
	}

	return string(out), nil
}
