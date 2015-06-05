package main

import (
	"fmt"
	"os"

	flag "github.com/docker/docker/pkg/mflag"
)

type command struct {
	name        string
	description string
}

type byName []command

var (
	flDaemon   = flag.Bool([]string{"d", "-daemon"}, false, "Enable daemon mode")
	flHost     = flag.String([]string{"H", "-host"}, "", "Daemon socket to connect to")
	flLogLevel = flag.String([]string{"l", "-log-level"}, "info", "Set the logging level")
	flDebug    = flag.Bool([]string{"D", "-debug"}, false, "Enable debug mode")
	flHelp     = flag.Bool([]string{"h", "-help"}, false, "Print usage")

	dnetCommands = []command{
		{"network", "Network management commands"},
	}
)

func init() {
	flag.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: dnet [OPTIONS] COMMAND [arg...]\n\nA self-sufficient runtime for container networking.\n\nOptions:\n")

		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, cmd := range dnetCommands {
			help += fmt.Sprintf("    %-10.10s%s\n", cmd.name, cmd.description)
		}

		help += "\nRun 'dnet COMMAND --help' for more information on a command."
		fmt.Fprintf(os.Stdout, "%s\n", help)
	}
}

func printUsage() {
	fmt.Println("Usage: dnet network <subcommand> <OPTIONS>")
}
