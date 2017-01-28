package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/commands"
	"github.com/docker/docker/pkg/term"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const descriptionSourcePath = "man/src/"

func generateCliYaml(opts *options) error {
	stdin, stdout, stderr := term.StdStreams()
	dockerCli := command.NewDockerCli(stdin, stdout, stderr)
	cmd := &cobra.Command{Use: "docker"}
	commands.AddCommands(cmd, dockerCli)
	source := filepath.Join(opts.source, descriptionSourcePath)
	if err := loadLongDescription(cmd, source); err != nil {
		return err
	}

	cmd.DisableAutoGenTag = true
	return GenYamlTree(cmd, opts.target)
}

func loadLongDescription(cmd *cobra.Command, path string) error {
	for _, cmd := range cmd.Commands() {
		if cmd.Name() == "" {
			continue
		}
		fullpath := filepath.Join(path, cmd.Name()+".md")

		if cmd.HasSubCommands() {
			loadLongDescription(cmd, filepath.Join(path, cmd.Name()))
		}

		if _, err := os.Stat(fullpath); err != nil {
			log.Printf("WARN: %s does not exist, skipping\n", fullpath)
			continue
		}

		content, err := ioutil.ReadFile(fullpath)
		if err != nil {
			return err
		}
		cmd.Long = string(content)
	}
	return nil
}

type options struct {
	source string
	target string
}

func parseArgs() (*options, error) {
	opts := &options{}
	cwd, _ := os.Getwd()
	flags := pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)
	flags.StringVar(&opts.source, "root", cwd, "Path to project root")
	flags.StringVar(&opts.target, "target", "/tmp", "Target path for generated yaml files")
	err := flags.Parse(os.Args[1:])
	return opts, err
}

func main() {
	opts, err := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	fmt.Printf("Project root: %s\n", opts.source)
	fmt.Printf("Generating yaml files into %s\n", opts.target)
	if err := generateCliYaml(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate yaml files: %s\n", err.Error())
	}
}
