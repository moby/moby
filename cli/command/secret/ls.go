package secret

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/spf13/cobra"
)

type listOptions struct {
	quiet bool
}

func newSecretListCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := listOptions{}

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List secrets",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretList(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Only display IDs")

	return cmd
}

func runSecretList(dockerCli *command.DockerCli, opts listOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	secrets, err := client.SecretList(ctx, types.SecretListOptions{})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(dockerCli.Out(), 20, 1, 3, ' ', 0)
	if opts.quiet {
		for _, s := range secrets {
			fmt.Fprintf(w, "%s\n", s.ID)
		}
	} else {
		fmt.Fprintf(w, "ID\tNAME\tCREATED\tUPDATED\tSIZE")
		fmt.Fprintf(w, "\n")

		for _, s := range secrets {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", s.ID, s.Spec.Annotations.Name, s.Meta.CreatedAt, s.Meta.UpdatedAt, s.SecretSize)
		}
	}

	w.Flush()

	return nil
}
