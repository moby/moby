package main

import (
	"sort"
	"testing"
)

// Tests if the subcommands of docker are sorted
func TestDockerSubcommandsAreSorted(t *testing.T) {
	if !sort.IsSorted(byName(dockerCommands)) {
		t.Fatal("Docker subcommands are not in sorted order")
	}
}
