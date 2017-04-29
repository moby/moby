package stack

import (
	"sort"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/formatter"
	"github.com/docker/docker/cli/compose/convert"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type listOptions struct {
	format string
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

	flags := cmd.Flags()
	flags.StringVar(&opts.format, "format", "", "Pretty-print stacks using a Go template")
	return cmd
}

func runList(dockerCli *command.DockerCli, opts listOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	stacks, err := getStacks(ctx, client)
	if err != nil {
		return err
	}
	format := opts.format
	if len(format) == 0 {
		format = formatter.TableFormatKey
	}
	stackCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: formatter.NewStackFormat(format),
	}
	sort.Sort(byName(stacks))
	return formatter.StackWrite(stackCtx, stacks)
}

type byName []*formatter.Stack

func (n byName) Len() int           { return len(n) }
func (n byName) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n byName) Less(i, j int) bool { return n[i].Name < n[j].Name }

func getStacks(ctx context.Context, apiclient client.APIClient) ([]*formatter.Stack, error) {
	services, err := apiclient.ServiceList(
		ctx,
		types.ServiceListOptions{Filters: getAllStacksFilter()})
	if err != nil {
		return nil, err
	}
	m := make(map[string]*formatter.Stack, 0)
	for _, service := range services {
		labels := service.Spec.Labels
		name, ok := labels[convert.LabelNamespace]
		if !ok {
			return nil, errors.Errorf("cannot get label %s for service %s",
				convert.LabelNamespace, service.ID)
		}
		ztack, ok := m[name]
		if !ok {
			m[name] = &formatter.Stack{
				Name:     name,
				Services: 1,
			}
		} else {
			ztack.Services++
		}
	}
	var stacks []*formatter.Stack
	for _, stack := range m {
		stacks = append(stacks, stack)
	}
	return stacks, nil
}
