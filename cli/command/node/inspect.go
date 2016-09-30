package node

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/inspect"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type inspectOptions struct {
	nodeIds []string
	format  string
	pretty  bool
}

func newInspectCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] self|NODE [NODE...]",
		Short: "Display detailed information on one or more nodes",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.nodeIds = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	flags.BoolVar(&opts.pretty, "pretty", false, "Print the information in a human friendly format.")
	return cmd
}

func runInspect(dockerCli *command.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	getRef := func(ref string) (interface{}, []byte, error) {
		nodeRef, err := Reference(ctx, client, ref)
		if err != nil {
			return nil, nil, err
		}
		node, _, err := client.NodeInspectWithRaw(ctx, nodeRef)
		return node, nil, err
	}

	if !opts.pretty {
		return inspect.Inspect(dockerCli.Out(), opts.nodeIds, opts.format, getRef)
	}
	return printHumanFriendly(dockerCli.Out(), opts.nodeIds, getRef)
}

func printHumanFriendly(out io.Writer, refs []string, getRef inspect.GetRefFunc) error {
	for idx, ref := range refs {
		obj, _, err := getRef(ref)
		if err != nil {
			return err
		}
		printNode(out, obj.(swarm.Node))

		// TODO: better way to do this?
		// print extra space between objects, but not after the last one
		if idx+1 != len(refs) {
			fmt.Fprintf(out, "\n\n")
		} else {
			fmt.Fprintf(out, "\n")
		}
	}
	return nil
}

// TODO: use a template
func printNode(out io.Writer, node swarm.Node) {
	fmt.Fprintf(out, "ID:\t\t\t%s\n", node.ID)
	ioutils.FprintfIfNotEmpty(out, "Name:\t\t\t%s\n", node.Spec.Name)
	if node.Spec.Labels != nil {
		fmt.Fprintln(out, "Labels:")
		for k, v := range node.Spec.Labels {
			fmt.Fprintf(out, " - %s = %s\n", k, v)
		}
	}

	ioutils.FprintfIfNotEmpty(out, "Hostname:\t\t%s\n", node.Description.Hostname)
	fmt.Fprintf(out, "Joined at:\t\t%s\n", command.PrettyPrint(node.CreatedAt))
	fmt.Fprintln(out, "Status:")
	fmt.Fprintf(out, " State:\t\t\t%s\n", command.PrettyPrint(node.Status.State))
	ioutils.FprintfIfNotEmpty(out, " Message:\t\t%s\n", command.PrettyPrint(node.Status.Message))
	fmt.Fprintf(out, " Availability:\t\t%s\n", command.PrettyPrint(node.Spec.Availability))

	if node.ManagerStatus != nil {
		fmt.Fprintln(out, "Manager Status:")
		fmt.Fprintf(out, " Address:\t\t%s\n", node.ManagerStatus.Addr)
		fmt.Fprintf(out, " Raft Status:\t\t%s\n", command.PrettyPrint(node.ManagerStatus.Reachability))
		leader := "No"
		if node.ManagerStatus.Leader {
			leader = "Yes"
		}
		fmt.Fprintf(out, " Leader:\t\t%s\n", leader)
	}

	fmt.Fprintln(out, "Platform:")
	fmt.Fprintf(out, " Operating System:\t%s\n", node.Description.Platform.OS)
	fmt.Fprintf(out, " Architecture:\t\t%s\n", node.Description.Platform.Architecture)

	fmt.Fprintln(out, "Resources:")
	fmt.Fprintf(out, " CPUs:\t\t\t%d\n", node.Description.Resources.NanoCPUs/1e9)
	fmt.Fprintf(out, " Memory:\t\t%s\n", units.BytesSize(float64(node.Description.Resources.MemoryBytes)))

	var pluginTypes []string
	pluginNamesByType := map[string][]string{}
	for _, p := range node.Description.Engine.Plugins {
		// append to pluginTypes only if not done previously
		if _, ok := pluginNamesByType[p.Type]; !ok {
			pluginTypes = append(pluginTypes, p.Type)
		}
		pluginNamesByType[p.Type] = append(pluginNamesByType[p.Type], p.Name)
	}

	if len(pluginTypes) > 0 {
		fmt.Fprintln(out, "Plugins:")
		sort.Strings(pluginTypes) // ensure stable output
		for _, pluginType := range pluginTypes {
			fmt.Fprintf(out, "  %s:\t\t%s\n", pluginType, strings.Join(pluginNamesByType[pluginType], ", "))
		}
	}
	fmt.Fprintf(out, "Engine Version:\t\t%s\n", node.Description.Engine.EngineVersion)

	if len(node.Description.Engine.Labels) != 0 {
		fmt.Fprintln(out, "Engine Labels:")
		for k, v := range node.Description.Engine.Labels {
			fmt.Fprintf(out, " - %s = %s\n", k, v)
		}
	}
}
