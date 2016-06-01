package main

import (
	"sort"
	"testing"

	"github.com/docker/docker/cli"
)

// Tests if the subcommands of docker are sorted
func TestDockerSubcommandsAreSorted(t *testing.T) {
	if !sort.IsSorted(byName(cli.DockerCommandUsage)) {
		t.Fatal("Docker subcommands are not in sorted order")
	}
}
