// +build experimental

package main

import "sort"

func init() {
	dockerCommands = append(dockerCommands, command{"network", "Network management"})

	//Sorting logic required here to pass Command Sort Test.
	sort.Sort(byName(dockerCommands))
}
