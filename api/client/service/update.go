package service

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-connections/nat"
	shlex "github.com/flynn-archive/go-shlex"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := newServiceOptions()

	cmd := &cobra.Command{
		Use:   "update [OPTIONS] SERVICE",
		Short: "Update a service",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, cmd.Flags(), args[0])
		},
	}

	flags := cmd.Flags()
	flags.String("image", "", "Service image tag")
	flags.String("args", "", "Service command args")
	addServiceFlags(cmd, opts)
	flags.StringSlice(flagEnvRemove, []string{}, "Remove an environment variable")
	flags.StringSlice(flagLabelRemove, []string{}, "The key of a label to remove")
	flags.StringSlice(flagMountRemove, []string{}, "The mount target for a mount to remove")
	flags.StringSlice(flagPublishRemove, []string{}, "The target port to remove")
	flags.StringSlice(flagNetworkRemove, []string{}, "The name of a network to remove")
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, flags *pflag.FlagSet, serviceID string) error {
	apiClient := dockerCli.Client()
	ctx := context.Background()
	updateOpts := types.ServiceUpdateOptions{}

	service, _, err := apiClient.ServiceInspectWithRaw(ctx, serviceID)
	if err != nil {
		return err
	}

	err = updateService(flags, &service.Spec)
	if err != nil {
		return err
	}

	// only send auth if flag was set
	sendAuth, err := flags.GetBool(flagRegistryAuth)
	if err != nil {
		return err
	}
	if sendAuth {
		// Retrieve encoded auth token from the image reference
		// This would be the old image if it didn't change in this update
		image := service.Spec.TaskTemplate.ContainerSpec.Image
		encodedAuth, err := dockerCli.RetrieveAuthTokenFromImage(ctx, image)
		if err != nil {
			return err
		}
		updateOpts.EncodedRegistryAuth = encodedAuth
	}

	err = apiClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, updateOpts)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", serviceID)
	return nil
}

func updateService(flags *pflag.FlagSet, spec *swarm.ServiceSpec) error {

	updateString := func(flag string, field *string) {
		if flags.Changed(flag) {
			*field, _ = flags.GetString(flag)
		}
	}

	updateListOpts := func(flag string, field *[]string) {
		if flags.Changed(flag) {
			value := flags.Lookup(flag).Value.(*opts.ListOpts)
			*field = value.GetAll()
		}
	}

	updateInt64Value := func(flag string, field *int64) {
		if flags.Changed(flag) {
			*field = flags.Lookup(flag).Value.(int64Value).Value()
		}
	}

	updateDuration := func(flag string, field *time.Duration) {
		if flags.Changed(flag) {
			*field, _ = flags.GetDuration(flag)
		}
	}

	updateDurationOpt := func(flag string, field *time.Duration) {
		if flags.Changed(flag) {
			*field = *flags.Lookup(flag).Value.(*DurationOpt).Value()
		}
	}

	updateUint64 := func(flag string, field *uint64) {
		if flags.Changed(flag) {
			*field, _ = flags.GetUint64(flag)
		}
	}

	updateUint64Opt := func(flag string, field *uint64) {
		if flags.Changed(flag) {
			*field = *flags.Lookup(flag).Value.(*Uint64Opt).Value()
		}
	}

	cspec := &spec.TaskTemplate.ContainerSpec
	task := &spec.TaskTemplate

	taskResources := func() *swarm.ResourceRequirements {
		if task.Resources == nil {
			task.Resources = &swarm.ResourceRequirements{}
		}
		return task.Resources
	}

	updateString(flagName, &spec.Name)
	updateLabels(flags, &spec.Labels)
	updateString("image", &cspec.Image)
	updateStringToSlice(flags, "args", &cspec.Args)
	updateEnvironment(flags, &cspec.Env)
	updateString("workdir", &cspec.Dir)
	updateString(flagUser, &cspec.User)
	updateMounts(flags, &cspec.Mounts)

	if flags.Changed(flagLimitCPU) || flags.Changed(flagLimitMemory) {
		taskResources().Limits = &swarm.Resources{}
		updateInt64Value(flagLimitCPU, &task.Resources.Limits.NanoCPUs)
		updateInt64Value(flagLimitMemory, &task.Resources.Limits.MemoryBytes)

	}
	if flags.Changed(flagReserveCPU) || flags.Changed(flagReserveMemory) {
		taskResources().Reservations = &swarm.Resources{}
		updateInt64Value(flagReserveCPU, &task.Resources.Reservations.NanoCPUs)
		updateInt64Value(flagReserveMemory, &task.Resources.Reservations.MemoryBytes)
	}

	updateDurationOpt(flagStopGracePeriod, cspec.StopGracePeriod)

	if anyChanged(flags, flagRestartCondition, flagRestartDelay, flagRestartMaxAttempts, flagRestartWindow) {
		if task.RestartPolicy == nil {
			task.RestartPolicy = &swarm.RestartPolicy{}
		}

		if flags.Changed(flagRestartCondition) {
			value, _ := flags.GetString(flagRestartCondition)
			task.RestartPolicy.Condition = swarm.RestartPolicyCondition(value)
		}
		updateDurationOpt(flagRestartDelay, task.RestartPolicy.Delay)
		updateUint64Opt(flagRestartMaxAttempts, task.RestartPolicy.MaxAttempts)
		updateDurationOpt((flagRestartWindow), task.RestartPolicy.Window)
	}

	if flags.Changed(flagConstraint) {
		task.Placement = &swarm.Placement{}
		updateSlice(flagConstraint, &task.Placement.Constraints)
	}

	if err := updateReplicas(flags, &spec.Mode); err != nil {
		return err
	}

	if anyChanged(flags, flagUpdateParallelism, flagUpdateDelay) {
		if spec.UpdateConfig == nil {
			spec.UpdateConfig = &swarm.UpdateConfig{}
		}
		updateUint64(flagUpdateParallelism, &spec.UpdateConfig.Parallelism)
		updateDuration(flagUpdateDelay, &spec.UpdateConfig.Delay)
	}

	updateNetworks(flags, &spec.Networks)
	if flags.Changed(flagEndpointMode) {
		value, _ := flags.GetString(flagEndpointMode)
		spec.EndpointSpec.Mode = swarm.ResolutionMode(value)
	}

	if flags.Changed(flagPublish) {
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		updatePorts(flags, &spec.EndpointSpec.Ports)
	}
	return nil
}

func updateStringToSlice(flags *pflag.FlagSet, flag string, field *[]string) error {
	if !flags.Changed(flag) {
		return nil
	}

	value, _ := flags.GetString(flag)
	valueSlice, err := shlex.Split(value)
	*field = valueSlice
	return err
}

func anyChanged(flags *pflag.FlagSet, fields ...string) bool {
	for _, flag := range fields {
		if flags.Changed(flag) {
			return true
		}
	}
	return false
}

func updateLabels(flags *pflag.FlagSet, field *map[string]string) {
	if flags.Changed(flagLabel) {
		if field == nil {
			*field = map[string]string{}
		}

		values := flags.Lookup(flagLabel).Value.(*opts.ListOpts).GetAll()
		for key, value := range runconfigopts.ConvertKVStringsToMap(values) {
			(*field)[key] = value
		}
	}

	if field != nil && flags.Changed(flagLabelRemove) {
		toRemove, _ := flags.GetStringSlice(flagLabelRemove)
		for _, label := range toRemove {
			delete(*field, label)
		}
	}
}

func updateEnvironment(flags *pflag.FlagSet, field *[]string) {
	if flags.Changed(flagEnv) {
		value := flags.Lookup(flagEnv).Value.(*opts.ListOpts)
		*field = append(*field, value.GetAll()...)
	}
	if flags.Changed(flagEnvRemove) {
		toRemove, _ := flags.GetStringSlice(flagEnvRemove)
		for _, envRemove := range toRemove {
			for i, env := range *field {
				key := envKey(env)
				if key == envRemove {
					*field = append((*field)[:i], (*field)[i+1:]...)
				}
			}
		}
	}
}

func envKey(value string) string {
	kv := strings.SplitN(value, "=", 2)
	return kv[0]
}

func updateMounts(flags *pflag.FlagSet, mounts *[]swarm.Mount) {
	if flags.Changed(flagMount) {
		values := flags.Lookup(flagMount).Value.(*MountOpt).Value()
		*mounts = append(*mounts, values...)
	}
	if flags.Changed(flagMountRemove) {
		toRemove, _ := flags.GetStringSlice(flagMountRemove)
		for _, mountTarget := range toRemove {
			for i, mount := range *mounts {
				if mount.Target == mountTarget {
					*mounts = append((*mounts)[:i], (*mounts)[i+1:]...)
				}
			}
		}
	}
}

func updatePorts(flags *pflag.FlagSet, portConfig *[]swarm.PortConfig) {
	if flags.Changed(flagPublish) {
		values := flags.Lookup(flagPublish).Value.(*opts.ListOpts).GetAll()
		ports, portBindings, _ := nat.ParsePortSpecs(values)

		for port := range ports {
			*portConfig = append(*portConfig, convertPortToPortConfig(port, portBindings)...)
		}
	}

	if flags.Changed(flagPublishRemove) {
		toRemove, _ := flags.GetStringSlice(flagPublishRemove)
		for _, rawTargetPort := range toRemove {
			targetPort := nat.Port(rawTargetPort)
			for i, port := range *portConfig {
				if string(port.Protocol) == targetPort.Proto() &&
					port.TargetPort == uint32(targetPort.Int()) {
					*portConfig = append((*portConfig)[:i], (*portConfig)[i+1:]...)
				}
			}
		}
	}
}

func updateNetworks(flags *pflag.FlagSet, attachments *[]swarm.NetworkAttachmentConfig) {
	if flags.Changed(flagNetwork) {
		networks, _ := flags.GetStringSlice(flagNetwork)
		for _, network := range networks {
			*attachments = append(*attachments, swarm.NetworkAttachmentConfig{Target: network})
		}
	}
	if flags.Changed(flagNetworkRemove) {
		toRemove, _ := flags.GetStringSlice(flagNetworkRemove)
		for _, networkTarget := range toRemove {
			for i, network := range *attachments {
				if network.Target == networkTarget {
					*attachments = append((*attachments)[:i], (*attachments)[i+1:]...)
				}
			}
		}
	}
}

func updateReplicas(flags *pflag.FlagSet, serviceMode *swarm.ServiceMode) error {
	if !flags.Changed(flagReplicas) {
		return nil
	}

	if serviceMode.Replicated == nil {
		return fmt.Errorf("replicas can only be used with replicated mode")
	}
	serviceMode.Replicated.Replicas = flags.Lookup(flagReplicas).Value.(*Uint64Opt).Value()
	return nil
}
