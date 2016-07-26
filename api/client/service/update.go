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

	flags.Var(newListOptsVar(), flagEnvRemove, "Remove an environment variable")
	flags.Var(newListOptsVar(), flagLabelRemove, "Remove a label by its key")
	flags.Var(newListOptsVar(), flagContainerLabelRemove, "Remove a container label by its key")
	flags.Var(newListOptsVar(), flagMountRemove, "Remove a mount by its target path")
	flags.Var(newListOptsVar(), flagPublishRemove, "Remove a published port by its target port")
	flags.Var(newListOptsVar(), flagNetworkRemove, "Remove a network by name")
	flags.Var(newListOptsVar(), flagConstraintRemove, "Remove a constraint")
	flags.Var(&opts.labels, flagLabelAdd, "Add or update service labels")
	flags.Var(&opts.containerLabels, flagContainerLabelAdd, "Add or update container labels")
	flags.Var(&opts.env, flagEnvAdd, "Add or update environment variables")
	flags.Var(&opts.mounts, flagMountAdd, "Add or update a mount on a service")
	flags.StringSliceVar(&opts.constraints, flagConstraintAdd, []string{}, "Add or update placement constraints")
	flags.StringSliceVar(&opts.networks, flagNetworkAdd, []string{}, "Add or update network attachments")
	flags.Var(&opts.endpoint.ports, flagPublishAdd, "Add or update a published port")
	return cmd
}

func newListOptsVar() *opts.ListOpts {
	return opts.NewListOptsRef(&[]string{}, nil)
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

	updateDurationOpt := func(flag string, field **time.Duration) {
		if flags.Changed(flag) {
			val := *flags.Lookup(flag).Value.(*DurationOpt).Value()
			*field = &val
		}
	}

	updateUint64 := func(flag string, field *uint64) {
		if flags.Changed(flag) {
			*field, _ = flags.GetUint64(flag)
		}
	}

	updateUint64Opt := func(flag string, field **uint64) {
		if flags.Changed(flag) {
			val := *flags.Lookup(flag).Value.(*Uint64Opt).Value()
			*field = &val
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
	updateContainerLabels(flags, &cspec.Labels)
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

	updateDurationOpt(flagStopGracePeriod, &cspec.StopGracePeriod)

	if anyChanged(flags, flagRestartCondition, flagRestartDelay, flagRestartMaxAttempts, flagRestartWindow) {
		if task.RestartPolicy == nil {
			task.RestartPolicy = &swarm.RestartPolicy{}
		}

		if flags.Changed(flagRestartCondition) {
			value, _ := flags.GetString(flagRestartCondition)
			task.RestartPolicy.Condition = swarm.RestartPolicyCondition(value)
		}
		updateDurationOpt(flagRestartDelay, &task.RestartPolicy.Delay)
		updateUint64Opt(flagRestartMaxAttempts, &task.RestartPolicy.MaxAttempts)
		updateDurationOpt(flagRestartWindow, &task.RestartPolicy.Window)
	}

	if anyChanged(flags, flagConstraintAdd, flagConstraintRemove) {
		if task.Placement == nil {
			task.Placement = &swarm.Placement{}
		}
		updatePlacement(flags, task.Placement)
	}

	if err := updateReplicas(flags, &spec.Mode); err != nil {
		return err
	}

	if anyChanged(flags, flagUpdateParallelism, flagUpdateDelay, flagUpdateFailureAction) {
		if spec.UpdateConfig == nil {
			spec.UpdateConfig = &swarm.UpdateConfig{}
		}
		updateUint64(flagUpdateParallelism, &spec.UpdateConfig.Parallelism)
		updateDuration(flagUpdateDelay, &spec.UpdateConfig.Delay)
		updateString(flagUpdateFailureAction, &spec.UpdateConfig.FailureAction)
	}

	updateNetworks(flags, &spec.Networks)
	if flags.Changed(flagEndpointMode) {
		value, _ := flags.GetString(flagEndpointMode)
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		spec.EndpointSpec.Mode = swarm.ResolutionMode(value)
	}

	if anyChanged(flags, flagPublishAdd, flagPublishRemove) {
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		updatePorts(flags, &spec.EndpointSpec.Ports)
	}

	if err := updateLogDriver(flags, &spec.TaskTemplate); err != nil {
		return err
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

func updatePlacement(flags *pflag.FlagSet, placement *swarm.Placement) {
	field, _ := flags.GetStringSlice(flagConstraintAdd)
	placement.Constraints = append(placement.Constraints, field...)

	toRemove := buildToRemoveSet(flags, flagConstraintRemove)
	placement.Constraints = removeItems(placement.Constraints, toRemove, itemKey)
}

func updateContainerLabels(flags *pflag.FlagSet, field *map[string]string) {
	if flags.Changed(flagContainerLabelAdd) {
		if *field == nil {
			*field = map[string]string{}
		}

		values := flags.Lookup(flagContainerLabelAdd).Value.(*opts.ListOpts).GetAll()
		for key, value := range runconfigopts.ConvertKVStringsToMap(values) {
			(*field)[key] = value
		}
	}

	if *field != nil && flags.Changed(flagContainerLabelRemove) {
		toRemove := flags.Lookup(flagContainerLabelRemove).Value.(*opts.ListOpts).GetAll()
		for _, label := range toRemove {
			delete(*field, label)
		}
	}
}

func updateLabels(flags *pflag.FlagSet, field *map[string]string) {
	if flags.Changed(flagLabelAdd) {
		if *field == nil {
			*field = map[string]string{}
		}

		values := flags.Lookup(flagLabelAdd).Value.(*opts.ListOpts).GetAll()
		for key, value := range runconfigopts.ConvertKVStringsToMap(values) {
			(*field)[key] = value
		}
	}

	if *field != nil && flags.Changed(flagLabelRemove) {
		toRemove := flags.Lookup(flagLabelRemove).Value.(*opts.ListOpts).GetAll()
		for _, label := range toRemove {
			delete(*field, label)
		}
	}
}

func updateEnvironment(flags *pflag.FlagSet, field *[]string) {
	if flags.Changed(flagEnvAdd) {
		value := flags.Lookup(flagEnvAdd).Value.(*opts.ListOpts)
		*field = append(*field, value.GetAll()...)
	}
	toRemove := buildToRemoveSet(flags, flagEnvRemove)
	*field = removeItems(*field, toRemove, envKey)
}

func envKey(value string) string {
	kv := strings.SplitN(value, "=", 2)
	return kv[0]
}

func itemKey(value string) string {
	return value
}

func buildToRemoveSet(flags *pflag.FlagSet, flag string) map[string]struct{} {
	var empty struct{}
	toRemove := make(map[string]struct{})

	if !flags.Changed(flag) {
		return toRemove
	}

	toRemoveSlice := flags.Lookup(flag).Value.(*opts.ListOpts).GetAll()
	for _, key := range toRemoveSlice {
		toRemove[key] = empty
	}
	return toRemove
}

func removeItems(
	seq []string,
	toRemove map[string]struct{},
	keyFunc func(string) string,
) []string {
	newSeq := []string{}
	for _, item := range seq {
		if _, exists := toRemove[keyFunc(item)]; !exists {
			newSeq = append(newSeq, item)
		}
	}
	return newSeq
}

func updateMounts(flags *pflag.FlagSet, mounts *[]swarm.Mount) {
	if flags.Changed(flagMountAdd) {
		values := flags.Lookup(flagMountAdd).Value.(*MountOpt).Value()
		*mounts = append(*mounts, values...)
	}
	toRemove := buildToRemoveSet(flags, flagMountRemove)

	newMounts := []swarm.Mount{}
	for _, mount := range *mounts {
		if _, exists := toRemove[mount.Target]; !exists {
			newMounts = append(newMounts, mount)
		}
	}
	*mounts = newMounts
}

func updatePorts(flags *pflag.FlagSet, portConfig *[]swarm.PortConfig) {
	if flags.Changed(flagPublishAdd) {
		values := flags.Lookup(flagPublishAdd).Value.(*opts.ListOpts).GetAll()
		ports, portBindings, _ := nat.ParsePortSpecs(values)

		for port := range ports {
			*portConfig = append(*portConfig, convertPortToPortConfig(port, portBindings)...)
		}
	}

	if !flags.Changed(flagPublishRemove) {
		return
	}
	toRemove := flags.Lookup(flagPublishRemove).Value.(*opts.ListOpts).GetAll()
	newPorts := []swarm.PortConfig{}
portLoop:
	for _, port := range *portConfig {
		for _, rawTargetPort := range toRemove {
			targetPort := nat.Port(rawTargetPort)
			if equalPort(targetPort, port) {
				continue portLoop
			}
		}
		newPorts = append(newPorts, port)
	}
	*portConfig = newPorts
}

func equalPort(targetPort nat.Port, port swarm.PortConfig) bool {
	return (string(port.Protocol) == targetPort.Proto() &&
		port.TargetPort == uint32(targetPort.Int()))
}

func updateNetworks(flags *pflag.FlagSet, attachments *[]swarm.NetworkAttachmentConfig) {
	if flags.Changed(flagNetworkAdd) {
		networks, _ := flags.GetStringSlice(flagNetworkAdd)
		for _, network := range networks {
			*attachments = append(*attachments, swarm.NetworkAttachmentConfig{Target: network})
		}
	}
	toRemove := buildToRemoveSet(flags, flagNetworkRemove)
	newNetworks := []swarm.NetworkAttachmentConfig{}
	for _, network := range *attachments {
		if _, exists := toRemove[network.Target]; !exists {
			newNetworks = append(newNetworks, network)
		}
	}
	*attachments = newNetworks
}

func updateReplicas(flags *pflag.FlagSet, serviceMode *swarm.ServiceMode) error {
	if !flags.Changed(flagReplicas) {
		return nil
	}

	if serviceMode == nil || serviceMode.Replicated == nil {
		return fmt.Errorf("replicas can only be used with replicated mode")
	}
	serviceMode.Replicated.Replicas = flags.Lookup(flagReplicas).Value.(*Uint64Opt).Value()
	return nil
}

// updateLogDriver updates the log driver only if the log driver flag is set.
// All options will be replaced with those provided on the command line.
func updateLogDriver(flags *pflag.FlagSet, taskTemplate *swarm.TaskSpec) error {
	if !flags.Changed(flagLogDriver) {
		return nil
	}

	name, err := flags.GetString(flagLogDriver)
	if err != nil {
		return err
	}

	if name == "" {
		return nil
	}

	taskTemplate.LogDriver = &swarm.Driver{
		Name:    name,
		Options: runconfigopts.ConvertKVStringsToMap(flags.Lookup(flagLogOpt).Value.(*opts.ListOpts).GetAll()),
	}

	return nil
}
