package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	apiclient "github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
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
		Use:   "inspect [OPTIONS] SERVICE [SERVICE...]",
		Short: "Display detailed information on one or more services",
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
	flags.StringVarP(&opts.format, "format", "f", "", "Format the output using the given go template")
	flags.BoolVar(&opts.pretty, "pretty", false, "Print the information in a human friendly format.")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	getRef := func(ref string) (interface{}, []byte, error) {
		service, _, err := client.ServiceInspectWithRaw(ctx, ref)
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
		fmt.Fprintln(out, "Mode:\t\tGlobal")
	} else {
		fmt.Fprintln(out, "Mode:\t\tReplicated")
		if service.Spec.Mode.Replicated.Replicas != nil {
			fmt.Fprintf(out, " Replicas:\t%d\n", *service.Spec.Mode.Replicated.Replicas)
		}
	}

	if service.UpdateStatus.State != "" {
		fmt.Fprintln(out, "Update status:")
		fmt.Fprintf(out, " State:\t\t%s\n", service.UpdateStatus.State)
		fmt.Fprintf(out, " Started:\t%s ago\n", strings.ToLower(units.HumanDuration(time.Since(service.UpdateStatus.StartedAt))))
		if service.UpdateStatus.State == swarm.UpdateStateCompleted {
			fmt.Fprintf(out, " Completed:\t%s ago\n", strings.ToLower(units.HumanDuration(time.Since(service.UpdateStatus.CompletedAt))))
		}
		fmt.Fprintf(out, " Message:\t%s\n", service.UpdateStatus.Message)
	}

	fmt.Fprintln(out, "Placement:")
	if service.Spec.TaskTemplate.Placement != nil && len(service.Spec.TaskTemplate.Placement.Constraints) > 0 {
		ioutils.FprintfIfNotEmpty(out, " Constraints\t: %s\n", strings.Join(service.Spec.TaskTemplate.Placement.Constraints, ", "))
	}
	fmt.Fprintf(out, "UpdateConfig:\n")
	fmt.Fprintf(out, " Parallelism:\t%d\n", service.Spec.UpdateConfig.Parallelism)
	if service.Spec.UpdateConfig.Delay.Nanoseconds() > 0 {
		fmt.Fprintf(out, " Delay:\t\t%s\n", service.Spec.UpdateConfig.Delay)
	}
	fmt.Fprintf(out, " On failure:\t%s\n", service.Spec.UpdateConfig.FailureAction)
	fmt.Fprintf(out, "ContainerSpec:\n")
	printContainerSpec(out, service.Spec.TaskTemplate.ContainerSpec)

	resources := service.Spec.TaskTemplate.Resources
	if resources != nil {
		fmt.Fprintln(out, "Resources:")
		printResources := func(out io.Writer, requirement string, r *swarm.Resources) {
			if r == nil || (r.MemoryBytes == 0 && r.NanoCPUs == 0) {
				return
			}
			fmt.Fprintf(out, " %s:\n", requirement)
			if r.NanoCPUs != 0 {
				fmt.Fprintf(out, "  CPU:\t\t%g\n", float64(r.NanoCPUs)/1e9)
			}
			if r.MemoryBytes != 0 {
				fmt.Fprintf(out, "  Memory:\t%s\n", units.BytesSize(float64(r.MemoryBytes)))
			}
		}
		printResources(out, "Reservations", resources.Reservations)
		printResources(out, "Limits", resources.Limits)
	}
	if len(service.Spec.Networks) > 0 {
		fmt.Fprintf(out, "Networks:")
		for _, n := range service.Spec.Networks {
			fmt.Fprintf(out, " %s", n.Target)
		}
	}

	if len(service.Endpoint.Ports) > 0 {
		fmt.Fprintln(out, "Ports:")
		for _, port := range service.Endpoint.Ports {
			ioutils.FprintfIfNotEmpty(out, " Name = %s\n", port.Name)
			fmt.Fprintf(out, " Protocol = %s\n", port.Protocol)
			fmt.Fprintf(out, " TargetPort = %d\n", port.TargetPort)
			fmt.Fprintf(out, " PublishedPort = %d\n", port.PublishedPort)
		}
	}
}

func printContainerSpec(out io.Writer, containerSpec swarm.ContainerSpec) {
	fmt.Fprintf(out, " Image:\t\t%s\n", containerSpec.Image)
	if len(containerSpec.Args) > 0 {
		fmt.Fprintf(out, " Args:\t\t%s\n", strings.Join(containerSpec.Args, " "))
	}
	if len(containerSpec.Env) > 0 {
		fmt.Fprintf(out, " Env:\t\t%s\n", strings.Join(containerSpec.Env, " "))
	}
	ioutils.FprintfIfNotEmpty(out, " Dir\t\t%s\n", containerSpec.Dir)
	ioutils.FprintfIfNotEmpty(out, " User\t\t%s\n", containerSpec.User)
	if len(containerSpec.Mounts) > 0 {
		fmt.Fprintln(out, " Mounts:")
		for _, v := range containerSpec.Mounts {
			fmt.Fprintf(out, "  Target = %s\n", v.Target)
			fmt.Fprintf(out, "  Source = %s\n", v.Source)
			fmt.Fprintf(out, "  ReadOnly = %v\n", v.ReadOnly)
			fmt.Fprintf(out, "  Type = %v\n", v.Type)
		}
	}
}
