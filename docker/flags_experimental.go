// +build experimental

package main

import (
	"sort"

	"github.com/docker/docker/cli"
)

func init() {
	experimentalCommands := []cli.Command{
		{"checkpoint", "Checkpoint one or more running containers"},
		{"restore", "Restore one or more checkpointed containers"},
	}

	dockerCommands = append(dockerCommands, experimentalCommands...)

	//Sorting logic required here to pass Command Sort Test.
	sort.Sort(byName(dockerCommands))
}
