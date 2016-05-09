package main

import (
	"sort"

	"github.com/docker/docker/cli"
)

type byName []cli.Command

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// TODO(tiborvass): do not show 'daemon' on client-only binaries

func sortCommands(commands []cli.Command) []cli.Command {
	dockerCommands := make([]cli.Command, len(commands))
	copy(dockerCommands, commands)
	sort.Sort(byName(dockerCommands))
	return dockerCommands
}
