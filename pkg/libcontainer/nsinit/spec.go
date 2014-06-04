package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

var specCommand = cli.Command{
	Name:   "spec",
	Usage:  "display the container specification",
	Action: specAction,
}

func specAction(context *cli.Context) {
	// returns the spec of the current container.
	spec, err := getContainerSpec(container)
	if err != nil {
		log.Printf("Failed to get spec - %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Spec:\n%v\n", spec)
	os.Exit(0)

}

// returns the container spec in json format.
func getContainerSpec(container *libcontainer.Container) (string, error) {
	spec, err := json.MarshalIndent(container, "", "\t")
	if err != nil {
		return "", err
	}
	return string(spec), nil
}
