package service

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	apiclient "github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	refs   []string
	format string
	pretty bool
}

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] SERVICE|TASK [SERVICE|TASK...]",
		Short: "Inspect a service or service tasks",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.refs = args

			if opts.pretty && len(opts.format) > 0 {
				return fmt.Errorf("--format is incompatible with human friendly format")
			}
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.Bool("help", false, "Print usage")
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	flags.BoolVarP(&opts.pretty, "pretty", "p", false, "Print the information in a human friendly format.")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	getRef := func(ref string) (interface{}, []byte, error) {
		service, err := client.ServiceInspect(ctx, ref)
		if err == nil || !apiclient.IsErrServiceNotFound(err) {
			return service, nil, err
		}
		return nil, nil, fmt.Errorf("Error: no such service: %s", ref)
	}

	if !opts.pretty {
		return inspect.Inspect(dockerCli.Out(), opts.refs, opts.format, getRef)
	}

	return printHumanFriendly(dockerCli.Out(), opts.refs, getRef)
}

func printHumanFriendly(out io.Writer, refs []string, getRef inspect.GetRefFunc) error {
	for idx, ref := range refs {
		obj, _, err := getRef(ref)
		if err != nil {
			return err
		}
		printService(out, obj.(swarm.Service))

		// TODO: better way to do this?
		// print extra space between objects, but not after the last one
		if idx+1 != len(refs) {
			fmt.Fprintf(out, "\n\n")
		}
	}
	return nil
}

// TODO: use a template
func printService(out io.Writer, service swarm.Service) {
	fmt.Fprintf(out, "ID:\t\t%s\n", service.ID)
	fmt.Fprintf(out, "Name:\t\t%s\n", service.Spec.Name)
	if service.Spec.Labels != nil {
		fmt.Fprintln(out, "Labels:")
		for k, v := range service.Spec.Labels {
			fmt.Fprintf(out, " - %s=%s\n", k, v)
		}
	}

	if service.Spec.Mode.Global != nil {
		fmt.Fprintln(out, "Mode:\t\tGLOBAL")
	} else {
		fmt.Fprintln(out, "Mode:\t\tREPLICATED")
		fmt.Fprintf(out, " Scale:\t\t%d\n", service.Spec.Mode.Replicated.Instances)
	}
	fmt.Fprintln(out, "Placement:")
	fmt.Fprintln(out, " Strategy:\tSPREAD")
	fmt.Fprintf(out, "UpateConfig:\n")
	fmt.Fprintf(out, " Parallelism:\t%d\n", service.Spec.UpdateConfig.Parallelism)
	if service.Spec.UpdateConfig.Delay.Nanoseconds() > 0 {
		fmt.Fprintf(out, " Delay:\t\t%s\n", service.Spec.UpdateConfig.Delay)
	}
	fmt.Fprintf(out, "ContainerSpec:\n")
	printContainerSpec(out, service.Spec.TaskSpec.ContainerSpec)
}

func printContainerSpec(out io.Writer, containerSpec swarm.ContainerSpec) {
	fmt.Fprintf(out, " Image:\t\t%s\n", containerSpec.Image)
	if len(containerSpec.Command) > 0 {
		fmt.Fprintf(out, " Command:\t%s\n", strings.Join(containerSpec.Command, " "))
	}
	if len(containerSpec.Args) > 0 {
		fmt.Fprintf(out, " Args:\t%s\n", strings.Join(containerSpec.Args, " "))
	}
	if len(containerSpec.Env) > 0 {
		fmt.Fprintf(out, " Env:\t\t%s\n", strings.Join(containerSpec.Env, " "))
	}
	ioutils.FprintfIfNotEmpty(out, " Dir\t\t%s\n", containerSpec.Dir)
	ioutils.FprintfIfNotEmpty(out, " User\t\t%s\n", containerSpec.User)
}
