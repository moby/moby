package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
)

func main() {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()

	logrus.SetOutput(stderr)

	flag.Merge(flag.CommandLine, clientFlags.FlagSet, commonFlags.FlagSet)

	flag.Usage = func() {
		fmt.Fprint(stdout, "Usage: docker [OPTIONS] COMMAND [arg...]\n       docker [ --help | -v | --version ]\n\n")
		fmt.Fprint(stdout, "A self-sufficient runtime for containers.\n\nOptions:\n")

		flag.CommandLine.SetOutput(stdout)
		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, cmd := range dockerCommands {
			help += fmt.Sprintf("    %-10.10s%s\n", cmd.Name, cmd.Description)
		}

		help += "\nRun 'docker COMMAND --help' for more information on a command."
		fmt.Fprintf(stdout, "%s\n", help)
	}

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}

	if *flHelp {
		// if global flag --help is present, regardless of what other options and commands there are,
		// just print the usage.
		flag.Usage()
		return
	}

	clientCli := client.NewDockerCli(stdin, stdout, stderr, clientFlags)

	c := cli.New(clientCli, NewDaemonProxy())
	if err := c.Run(flag.Args()...); err != nil {
		if sterr, ok := err.(cli.StatusError); ok {
			if sterr.Status != "" {
				fmt.Fprintln(stderr, sterr.Status)
				os.Exit(1)
			}
			os.Exit(sterr.StatusCode)
		}
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Docker version %s, build %s, experimental\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}
