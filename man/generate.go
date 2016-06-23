package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/command"
	"github.com/docker/docker/pkg/term"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func generateManPages(path string) error {
	header := &doc.GenManHeader{
		Title:   "DOCKER",
		Section: "1",
		Source:  "Docker Community",
	}

	stdin, stdout, stderr := term.StdStreams()
	dockerCli := client.NewDockerCli(stdin, stdout, stderr)
	cmd := &cobra.Command{Use: "docker"}
	command.AddCommands(cmd, dockerCli)

	cmd.DisableAutoGenTag = true
	return doc.GenManTreeFromOpts(cmd, doc.GenManTreeOptions{
		Header:           header,
		Path:             path,
		CommandSeparator: "-",
	})
}

func main() {
	path := "/tmp"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	fmt.Printf("Generating man pages into %s\n", path)
	if err := generateManPages(path); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate man pages: %s\n", err.Error())
	}
}
