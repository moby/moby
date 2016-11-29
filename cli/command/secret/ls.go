package secret

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type listOptions struct {
	quiet bool
}

func newSecretListCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := listOptions{}

	cmd := &cobra.Command{
		Use:     "ls [OPTIONS]",
		Aliases: []string{"list"},
		Short:   "List secrets",
		Args:    cli.NoArgs,
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
		fmt.Fprintf(w, "ID\tNAME\tCREATED\tUPDATED")
		fmt.Fprintf(w, "\n")

		for _, s := range secrets {
			created := units.HumanDuration(time.Now().UTC().Sub(s.Meta.CreatedAt)) + " ago"
			updated := units.HumanDuration(time.Now().UTC().Sub(s.Meta.UpdatedAt)) + " ago"

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Spec.Annotations.Name, created, updated)
		}
	}

	w.Flush()

	return nil
}
