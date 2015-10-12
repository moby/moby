package main

import (
	"sort"

	"github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

var (
	flHelp    = flag.Bool([]string{"h", "-help"}, false, "Print usage")
	flVersion = flag.Bool([]string{"v", "-version"}, false, "Print version information and quit")
)

type byName []cli.Command

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }

var dockerCommands []cli.Command

// TODO(tiborvass): do not show 'daemon' on client-only binaries

func init() {
	for _, cmd := range cli.DockerCommands {
		dockerCommands = append(dockerCommands, cmd)
	}
	sort.Sort(byName(dockerCommands))
}
