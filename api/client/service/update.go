package service

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := newServiceOptions()
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] SERVICE",
		Short: "Update a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, flags, args[0])
		},
	}

	flags = cmd.Flags()
	flags.String("image", "", "Service image tag")
	flags.StringSlice("command", []string{}, "Service command")
	flags.StringSlice("arg", []string{}, "Service command args")
	addServiceFlags(cmd, opts)
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, flags *pflag.FlagSet, serviceID string) error {
	client := dockerCli.Client()
	ctx := context.Background()

	service, err := client.ServiceInspect(ctx, serviceID)
	if err != nil {
		return err
	}

	err = mergeService(&service.Spec, flags)
	if err != nil {
		return err
	}
	err = client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", serviceID)
	return nil
}

func mergeService(spec *swarm.ServiceSpec, flags *pflag.FlagSet) error {

	mergeString := func(flag string, field *string) {
		if flags.Changed(flag) {
			*field, _ = flags.GetString(flag)
		}
	}

	mergeListOpts := func(flag string, field *[]string) {
		if flags.Changed(flag) {
			value := flags.Lookup(flag).Value.(*opts.ListOpts)
			*field = value.GetAll()
		}
	}

	mergeSlice := func(flag string, field *[]string) {
		if flags.Changed(flag) {
			*field, _ = flags.GetStringSlice(flag)
		}
	}

	mergeInt64Value := func(flag string, field *int64) {
		if flags.Changed(flag) {
			*field = flags.Lookup(flag).Value.(int64Value).Value()
		}
	}

	mergeDuration := func(flag string, field *time.Duration) {
		if flags.Changed(flag) {
			*field, _ = flags.GetDuration(flag)
		}
	}

	mergeDurationOpt := func(flag string, field *time.Duration) {
		if flags.Changed(flag) {
			*field = *flags.Lookup(flag).Value.(*DurationOpt).Value()
		}
	}

	mergeUint64 := func(flag string, field *uint64) {
		if flags.Changed(flag) {
			*field, _ = flags.GetUint64(flag)
		}
	}

	mergeUint64Opt := func(flag string, field *uint64) {
		if flags.Changed(flag) {
			*field = *flags.Lookup(flag).Value.(*Uint64Opt).Value()
		}
	}

	cspec := &spec.TaskTemplate.ContainerSpec
	task := &spec.TaskTemplate
	mergeString(flagName, &spec.Name)
	mergeLabels(flags, &spec.Labels)
	mergeString("image", &cspec.Image)
	mergeSlice("command", &cspec.Command)
	mergeSlice("arg", &cspec.Command)
	mergeListOpts("env", &cspec.Env)
	mergeString("workdir", &cspec.Dir)
	mergeString("user", &cspec.User)
	mergeMounts(flags, &cspec.Mounts)

	mergeInt64Value("limit-cpu", &task.Resources.Limits.NanoCPUs)
	mergeInt64Value("limit-memory", &task.Resources.Limits.MemoryBytes)
	mergeInt64Value("reserve-cpu", &task.Resources.Reservations.NanoCPUs)
	mergeInt64Value("reserve-memory", &task.Resources.Reservations.MemoryBytes)

	mergeDurationOpt("stop-grace-period", cspec.StopGracePeriod)

	if flags.Changed(flagRestartCondition) {
		value, _ := flags.GetString(flagRestartCondition)
		task.RestartPolicy.Condition = swarm.RestartPolicyCondition(value)
	}
	mergeDurationOpt("restart-delay", task.RestartPolicy.Delay)
	mergeUint64Opt("restart-max-attempts", task.RestartPolicy.MaxAttempts)
	mergeDurationOpt("restart-window", task.RestartPolicy.Window)
	mergeSlice("constraint", &task.Placement.Constraints)

	if err := mergeMode(flags, &spec.Mode); err != nil {
		return err
	}

	mergeUint64(flagUpdateParallelism, &spec.UpdateConfig.Parallelism)
	mergeDuration(flagUpdateDelay, &spec.UpdateConfig.Delay)

	mergeNetworks(flags, &spec.Networks)
	if flags.Changed(flagEndpointMode) {
		value, _ := flags.GetString(flagEndpointMode)
		spec.EndpointSpec.Mode = swarm.ResolutionMode(value)
	}

	mergePorts(flags, &spec.EndpointSpec.Ports)

	return nil
}

func mergeLabels(flags *pflag.FlagSet, field *map[string]string) {
	if !flags.Changed(flagLabel) {
		return
	}

	if *field == nil {
		*field = make(map[string]string)
	}

	values := flags.Lookup(flagLabel).Value.(*opts.ListOpts).GetAll()
	for key, value := range runconfigopts.ConvertKVStringsToMap(values) {
		(*field)[key] = value
	}
}

// TODO: should this override by destination path, or does swarm handle that?
func mergeMounts(flags *pflag.FlagSet, mounts *[]swarm.Mount) {
	if !flags.Changed(flagMount) {
		return
	}

	values := flags.Lookup(flagMount).Value.(*MountOpt).Value()
	*mounts = append(*mounts, values...)
}

// TODO: should this override by name, or does swarm handle that?
func mergePorts(flags *pflag.FlagSet, portConfig *[]swarm.PortConfig) {
	if !flags.Changed(flagPublish) {
		return
	}

	values := flags.Lookup(flagPublish).Value.(*opts.ListOpts).GetAll()
	ports, portBindings, _ := nat.ParsePortSpecs(values)

	for port := range ports {
		*portConfig = append(*portConfig, convertPortToPortConfig(port, portBindings)...)
	}
}

func mergeNetworks(flags *pflag.FlagSet, attachments *[]swarm.NetworkAttachmentConfig) {
	if !flags.Changed(flagNetwork) {
		return
	}
	networks, _ := flags.GetStringSlice(flagNetwork)
	for _, network := range networks {
		*attachments = append(*attachments, swarm.NetworkAttachmentConfig{Target: network})
	}
}

func mergeMode(flags *pflag.FlagSet, serviceMode *swarm.ServiceMode) error {
	if !flags.Changed(flagMode) && !flags.Changed(flagReplicas) {
		return nil
	}

	var mode string
	if flags.Changed(flagMode) {
		mode, _ = flags.GetString(flagMode)
	}

	if !(mode == "replicated" || serviceMode.Replicated != nil) && flags.Changed(flagReplicas) {
		return fmt.Errorf("replicas can only be used with replicated mode")
	}

	if mode == "global" {
		serviceMode.Replicated = nil
		serviceMode.Global = &swarm.GlobalService{}
		return nil
	}

	if flags.Changed(flagReplicas) {
		replicas := flags.Lookup(flagReplicas).Value.(*Uint64Opt).Value()
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: replicas}
		serviceMode.Global = nil
		return nil
	}

	if mode == "replicated" {
		if serviceMode.Replicated != nil {
			return nil
		}
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: &DefaultReplicas}
		serviceMode.Global = nil
	}

	return nil
}
