package checkpoint

import (
	"fmt"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type listOptions struct {
	checkpointDir string
}

func newListCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS] CONTAINER",
		Aliases: []string{"list"},
		Short:   "List checkpoints for a container",
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.checkpointDir, "checkpoint-dir", "", "", "Use a custom checkpoint storage directory")

	return cmd

}

func runList(dockerCli *command.DockerCli, container string, opts listOptions) error {
	client := dockerCli.Client()

	listOpts := types.CheckpointListOptions{
		CheckpointDir: opts.checkpointDir,
	}

	checkpoints, err := client.CheckpointList(context.Background(), container, listOpts)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "CHECKPOINT NAME")
	fmt.Fprintf(w, "\n")

	for _, checkpoint := range checkpoints {
		fmt.Fprintf(w, "%s\t", checkpoint.Name)
		fmt.Fprint(w, "\n")
	}

	w.Flush()
	return nil
}
