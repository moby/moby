package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/codegangsta/cli"
	"github.com/dotcloud/docker/pkg/libcontainer"
)

var specCommand = cli.Command{
	Name:   "spec",
	Usage:  "display the container specification",
	Action: specAction,
}

func specAction(context *cli.Context) {
	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	spec, err := getContainerSpec(container)
	if err != nil {
		log.Fatalf("Failed to get spec - %v\n", err)
	}

	fmt.Printf("Spec:\n%v\n", spec)
}

// returns the container spec in json format.
func getContainerSpec(container *libcontainer.Container) (string, error) {
	spec, err := json.MarshalIndent(container, "", "\t")
	if err != nil {
		return "", err
	}

	return string(spec), nil
}
