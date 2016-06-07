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

	cspec := &spec.TaskSpec.ContainerSpec
	task := &spec.TaskSpec
	mergeString("name", &spec.Name)
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

	if flags.Changed("restart-policy-condition") {
		value, _ := flags.GetString("restart-policy-condition")
		task.RestartPolicy.Condition = swarm.RestartPolicyCondition(value)
	}
	mergeDurationOpt("restart-policy-delay", task.RestartPolicy.Delay)
	mergeUint64Opt("restart-policy-max-attempts", task.RestartPolicy.MaxAttempts)
	mergeDurationOpt("restart-policy-window", task.RestartPolicy.Window)
	mergeSlice("constraint", &task.Placement.Constraints)

	if err := mergeMode(flags, &spec.Mode); err != nil {
		return err
	}

	mergeUint64("updateconfig-parallelism", &spec.UpdateConfig.Parallelism)
	mergeDuration("updateconfig-delay", &spec.UpdateConfig.Delay)

	mergeNetworks(flags, &spec.Networks)
	if flags.Changed("endpoint-mode") {
		value, _ := flags.GetString("endpoint-mode")
		spec.EndpointSpec.Mode = swarm.ResolutionMode(value)
	}
	if flags.Changed("endpoint-ingress") {
		value, _ := flags.GetString("endpoint-ingress")
		spec.EndpointSpec.Ingress = swarm.IngressRouting(value)
	}

	mergePorts(flags, &spec.EndpointSpec.ExposedPorts)

	return nil
}

func mergeLabels(flags *pflag.FlagSet, field *map[string]string) {
	if !flags.Changed("label") {
		return
	}

	if *field == nil {
		*field = make(map[string]string)
	}

	values := flags.Lookup("label").Value.(*opts.ListOpts).GetAll()
	for key, value := range runconfigopts.ConvertKVStringsToMap(values) {
		(*field)[key] = value
	}
}

// TODO: should this override by destination path, or does swarm handle that?
func mergeMounts(flags *pflag.FlagSet, mounts *[]swarm.Mount) {
	if !flags.Changed("volume") {
		return
	}

	values := flags.Lookup("volume").Value.(*opts.ListOpts).GetAll()
	*mounts = append(*mounts, ConvertMounts(values)...)
}

// TODO: should this override by name, or does swarm handle that?
func mergePorts(flags *pflag.FlagSet, portConfig *[]swarm.PortConfig) {
	if !flags.Changed("ports") {
		return
	}

	values := flags.Lookup("ports").Value.(*opts.ListOpts).GetAll()
	ports, portBindings, _ := nat.ParsePortSpecs(values)

	for port := range ports {
		*portConfig = append(*portConfig, convertPortToPortConfig(port, portBindings)...)
	}
}

func mergeNetworks(flags *pflag.FlagSet, attachments *[]swarm.NetworkAttachmentConfig) {
	if !flags.Changed("network") {
		return
	}
	networks, _ := flags.GetStringSlice("network")
	for _, network := range networks {
		*attachments = append(*attachments, swarm.NetworkAttachmentConfig{Target: network})
	}
}

func mergeMode(flags *pflag.FlagSet, serviceMode *swarm.ServiceMode) error {
	if !flags.Changed("mode") && !flags.Changed("scale") {
		return nil
	}

	var mode string
	if flags.Changed("mode") {
		mode, _ = flags.GetString("mode")
	}

	if !(mode == "replicated" || serviceMode.Replicated != nil) && flags.Changed("scale") {
		return fmt.Errorf("scale can only be used with replicated mode")
	}

	if mode == "global" {
		serviceMode.Replicated = nil
		serviceMode.Global = &swarm.GlobalService{}
		return nil
	}

	if flags.Changed("scale") {
		scale := flags.Lookup("scale").Value.(*Uint64Opt).Value()
		serviceMode.Replicated = &swarm.ReplicatedService{Instances: scale}
		serviceMode.Global = nil
		return nil
	}

	if mode == "replicated" {
		if serviceMode.Replicated != nil {
			return nil
		}
		serviceMode.Replicated = &swarm.ReplicatedService{Instances: &DefaultScale}
		serviceMode.Global = nil
	}

	return nil
}
