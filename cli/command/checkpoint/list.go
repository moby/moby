// +build experimental

package checkpoint

import (
	"fmt"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

func newListCommand(dockerCli *command.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:     "ls CONTAINER",
		Aliases: []string{"list"},
		Short:   "List checkpoints for a container",
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, args[0])
		},
	}
}

func runList(dockerCli *command.DockerCli, container string) error {
	client := dockerCli.Client()

	checkpoints, err := client.CheckpointList(context.Background(), container)
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
