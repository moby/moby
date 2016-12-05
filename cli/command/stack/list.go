package stack

import (
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/compose/convert"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

const (
	listItemFmt = "%s\t%s\n"
)

type listOptions struct {
}

func newListCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := listOptions{}

	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List stacks",
		Args:    cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, opts)
		},
	}

	return cmd
}

func runList(dockerCli *command.DockerCli, opts listOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	stacks, err := getStacks(ctx, client)
	if err != nil {
		return err
	}

	out := dockerCli.Out()
	printTable(out, stacks)
	return nil
}

func printTable(out io.Writer, stacks []*stack) {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)

	// Ignore flushing errors
	defer writer.Flush()

	fmt.Fprintf(writer, listItemFmt, "NAME", "SERVICES")
	for _, stack := range stacks {
		fmt.Fprintf(
			writer,
			listItemFmt,
			stack.Name,
			strconv.Itoa(stack.Services),
		)
	}
}

type stack struct {
	// Name is the name of the stack
	Name string
	// Services is the number of the services
	Services int
}

func getStacks(
	ctx context.Context,
	apiclient client.APIClient,
) ([]*stack, error) {
	services, err := apiclient.ServiceList(
		ctx,
		types.ServiceListOptions{Filters: getAllStacksFilter()})
	if err != nil {
		return nil, err
	}
	m := make(map[string]*stack, 0)
	for _, service := range services {
		labels := service.Spec.Labels
		name, ok := labels[convert.LabelNamespace]
		if !ok {
			return nil, fmt.Errorf("cannot get label %s for service %s",
				convert.LabelNamespace, service.ID)
		}
		ztack, ok := m[name]
		if !ok {
			m[name] = &stack{
				Name:     name,
				Services: 1,
			}
		} else {
			ztack.Services++
		}
	}
	var stacks []*stack
	for _, stack := range m {
		stacks = append(stacks, stack)
	}
	return stacks, nil
}
