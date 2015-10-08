// +build experimental

package main

import (
	"sort"

	"github.com/docker/docker/cli"
)

func init() {
	dockerCommands = append(dockerCommands, cli.Command{Name: "network", Description: "Network management"})

	//Sorting logic required here to pass Command Sort Test.
	sort.Sort(byName(dockerCommands))
}
