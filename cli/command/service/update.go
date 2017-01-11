package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/client"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/go-connections/nat"
	shlex "github.com/flynn-archive/go-shlex"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

func newUpdateCommand(dockerCli *command.DockerCli) *cobra.Command {
	serviceOpts := newServiceOptions()

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
	flags.Bool("rollback", false, "Rollback to previous specification")
	flags.Bool("force", false, "Force update even if no changes require it")
	addServiceFlags(cmd, serviceOpts)

	flags.Var(newListOptsVar(), flagEnvRemove, "Remove an environment variable")
	flags.Var(newListOptsVar(), flagGroupRemove, "Remove a previously added supplementary user group from the container")
	flags.Var(newListOptsVar(), flagLabelRemove, "Remove a label by its key")
	flags.Var(newListOptsVar(), flagContainerLabelRemove, "Remove a container label by its key")
	flags.Var(newListOptsVar(), flagMountRemove, "Remove a mount by its target path")
	// flags.Var(newListOptsVar().WithValidator(validatePublishRemove), flagPublishRemove, "Remove a published port by its target port")
	flags.Var(&opts.PortOpt{}, flagPublishRemove, "Remove a published port by its target port")
	flags.Var(newListOptsVar(), flagConstraintRemove, "Remove a constraint")
	flags.Var(newListOptsVar(), flagDNSRemove, "Remove a custom DNS server")
	flags.Var(newListOptsVar(), flagDNSOptionRemove, "Remove a DNS option")
	flags.Var(newListOptsVar(), flagDNSSearchRemove, "Remove a DNS search domain")
	flags.Var(newListOptsVar(), flagHostRemove, "Remove a custom host-to-IP mapping (host:ip)")
	flags.Var(&serviceOpts.labels, flagLabelAdd, "Add or update a service label")
	flags.Var(&serviceOpts.containerLabels, flagContainerLabelAdd, "Add or update a container label")
	flags.Var(&serviceOpts.env, flagEnvAdd, "Add or update an environment variable")
	flags.Var(newListOptsVar(), flagSecretRemove, "Remove a secret")
	flags.Var(&serviceOpts.secrets, flagSecretAdd, "Add or update a secret on a service")
	flags.Var(&serviceOpts.mounts, flagMountAdd, "Add or update a mount on a service")
	flags.Var(&serviceOpts.constraints, flagConstraintAdd, "Add or update a placement constraint")
	flags.Var(&serviceOpts.endpoint.publishPorts, flagPublishAdd, "Add or update a published port")
	flags.Var(&serviceOpts.groups, flagGroupAdd, "Add an additional supplementary user group to the container")
	flags.Var(&serviceOpts.dns, flagDNSAdd, "Add or update a custom DNS server")
	flags.Var(&serviceOpts.dnsOption, flagDNSOptionAdd, "Add or update a DNS option")
	flags.Var(&serviceOpts.dnsSearch, flagDNSSearchAdd, "Add or update a custom DNS search domain")
	flags.Var(&serviceOpts.hosts, flagHostAdd, "Add or update a custom host-to-IP mapping (host:ip)")

	return cmd
}

func newListOptsVar() *opts.ListOpts {
	return opts.NewListOptsRef(&[]string{}, nil)
}

func runUpdate(dockerCli *command.DockerCli, flags *pflag.FlagSet, serviceID string) error {
	apiClient := dockerCli.Client()
	ctx := context.Background()
	updateOpts := types.ServiceUpdateOptions{}

	service, _, err := apiClient.ServiceInspectWithRaw(ctx, serviceID)
	if err != nil {
		return err
	}

	rollback, err := flags.GetBool("rollback")
	if err != nil {
		return err
	}

	spec := &service.Spec
	if rollback {
		spec = service.PreviousSpec
		if spec == nil {
			return fmt.Errorf("service does not have a previous specification to roll back to")
		}
	}

	err = updateService(flags, spec)
	if err != nil {
		return err
	}

	if flags.Changed("image") {
		if err := resolveServiceImageDigest(dockerCli, spec); err != nil {
			return err
		}
	}

	updatedSecrets, err := getUpdatedSecrets(apiClient, flags, spec.TaskTemplate.ContainerSpec.Secrets)
	if err != nil {
		return err
	}

	spec.TaskTemplate.ContainerSpec.Secrets = updatedSecrets

	// only send auth if flag was set
	sendAuth, err := flags.GetBool(flagRegistryAuth)
	if err != nil {
		return err
	}
	if sendAuth {
		// Retrieve encoded auth token from the image reference
		// This would be the old image if it didn't change in this update
		image := spec.TaskTemplate.ContainerSpec.Image
		encodedAuth, err := command.RetrieveAuthTokenFromImage(ctx, dockerCli, image)
		if err != nil {
			return err
		}
		updateOpts.EncodedRegistryAuth = encodedAuth
	} else if rollback {
		updateOpts.RegistryAuthFrom = types.RegistryAuthFromPreviousSpec
	} else {
		updateOpts.RegistryAuthFrom = types.RegistryAuthFromSpec
	}

	response, err := apiClient.ServiceUpdate(ctx, service.ID, service.Version, *spec, updateOpts)
	if err != nil {
		return err
	}

	for _, warning := range response.Warnings {
		fmt.Fprintln(dockerCli.Err(), warning)
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

	updateFloatValue := func(flag string, field *float32) {
		if flags.Changed(flag) {
			*field = flags.Lookup(flag).Value.(*floatValue).Value()
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

	updateLabels(flags, &spec.Labels)
	updateContainerLabels(flags, &cspec.Labels)
	updateString("image", &cspec.Image)
	updateStringToSlice(flags, "args", &cspec.Args)
	updateEnvironment(flags, &cspec.Env)
	updateString(flagWorkdir, &cspec.Dir)
	updateString(flagUser, &cspec.User)
	updateString(flagHostname, &cspec.Hostname)
	if err := updateMounts(flags, &cspec.Mounts); err != nil {
		return err
	}

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

	if anyChanged(flags, flagUpdateParallelism, flagUpdateDelay, flagUpdateMonitor, flagUpdateFailureAction, flagUpdateMaxFailureRatio) {
		if spec.UpdateConfig == nil {
			spec.UpdateConfig = &swarm.UpdateConfig{}
		}
		updateUint64(flagUpdateParallelism, &spec.UpdateConfig.Parallelism)
		updateDuration(flagUpdateDelay, &spec.UpdateConfig.Delay)
		updateDuration(flagUpdateMonitor, &spec.UpdateConfig.Monitor)
		updateString(flagUpdateFailureAction, &spec.UpdateConfig.FailureAction)
		updateFloatValue(flagUpdateMaxFailureRatio, &spec.UpdateConfig.MaxFailureRatio)
	}

	if flags.Changed(flagEndpointMode) {
		value, _ := flags.GetString(flagEndpointMode)
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		spec.EndpointSpec.Mode = swarm.ResolutionMode(value)
	}

	if anyChanged(flags, flagGroupAdd, flagGroupRemove) {
		if err := updateGroups(flags, &cspec.Groups); err != nil {
			return err
		}
	}

	if anyChanged(flags, flagPublishAdd, flagPublishRemove) {
		if spec.EndpointSpec == nil {
			spec.EndpointSpec = &swarm.EndpointSpec{}
		}
		if err := updatePorts(flags, &spec.EndpointSpec.Ports); err != nil {
			return err
		}
	}

	if anyChanged(flags, flagDNSAdd, flagDNSRemove, flagDNSOptionAdd, flagDNSOptionRemove, flagDNSSearchAdd, flagDNSSearchRemove) {
		if cspec.DNSConfig == nil {
			cspec.DNSConfig = &swarm.DNSConfig{}
		}
		if err := updateDNSConfig(flags, &cspec.DNSConfig); err != nil {
			return err
		}
	}

	if anyChanged(flags, flagHostAdd, flagHostRemove) {
		if err := updateHosts(flags, &cspec.Hosts); err != nil {
			return err
		}
	}

	if err := updateLogDriver(flags, &spec.TaskTemplate); err != nil {
		return err
	}

	force, err := flags.GetBool("force")
	if err != nil {
		return err
	}

	if force {
		spec.TaskTemplate.ForceUpdate++
	}

	if err := updateHealthcheck(flags, cspec); err != nil {
		return err
	}

	if flags.Changed(flagTTY) {
		tty, err := flags.GetBool(flagTTY)
		if err != nil {
			return err
		}
		cspec.TTY = tty
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
	if flags.Changed(flagConstraintAdd) {
		values := flags.Lookup(flagConstraintAdd).Value.(*opts.ListOpts).GetAll()
		placement.Constraints = append(placement.Constraints, values...)
	}
	toRemove := buildToRemoveSet(flags, flagConstraintRemove)

	newConstraints := []string{}
	for _, constraint := range placement.Constraints {
		if _, exists := toRemove[constraint]; !exists {
			newConstraints = append(newConstraints, constraint)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConstraints)

	placement.Constraints = newConstraints
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
	envSet := map[string]string{}
	for _, v := range *field {
		envSet[envKey(v)] = v
	}
	if flags.Changed(flagEnvAdd) {
		value := flags.Lookup(flagEnvAdd).Value.(*opts.ListOpts)
		for _, v := range value.GetAll() {
			envSet[envKey(v)] = v
		}
	}

	*field = []string{}
	for _, v := range envSet {
		*field = append(*field, v)
	}

	toRemove := buildToRemoveSet(flags, flagEnvRemove)
	*field = removeItems(*field, toRemove, envKey)
}

func getUpdatedSecrets(apiClient client.SecretAPIClient, flags *pflag.FlagSet, secrets []*swarm.SecretReference) ([]*swarm.SecretReference, error) {
	newSecrets := []*swarm.SecretReference{}

	toRemove := buildToRemoveSet(flags, flagSecretRemove)
	for _, secret := range secrets {
		if _, exists := toRemove[secret.SecretName]; !exists {
			newSecrets = append(newSecrets, secret)
		}
	}

	if flags.Changed(flagSecretAdd) {
		values := flags.Lookup(flagSecretAdd).Value.(*opts.SecretOpt).Value()

		addSecrets, err := parseSecrets(apiClient, values)
		if err != nil {
			return nil, err
		}
		newSecrets = append(newSecrets, addSecrets...)
	}

	return newSecrets, nil
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

type byMountSource []mounttypes.Mount

func (m byMountSource) Len() int      { return len(m) }
func (m byMountSource) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m byMountSource) Less(i, j int) bool {
	a, b := m[i], m[j]

	if a.Source == b.Source {
		return a.Target < b.Target
	}

	return a.Source < b.Source
}

func updateMounts(flags *pflag.FlagSet, mounts *[]mounttypes.Mount) error {

	mountsByTarget := map[string]mounttypes.Mount{}

	if flags.Changed(flagMountAdd) {
		values := flags.Lookup(flagMountAdd).Value.(*opts.MountOpt).Value()
		for _, mount := range values {
			if _, ok := mountsByTarget[mount.Target]; ok {
				return fmt.Errorf("duplicate mount target")
			}
			mountsByTarget[mount.Target] = mount
		}
	}

	// Add old list of mount points minus updated one.
	for _, mount := range *mounts {
		if _, ok := mountsByTarget[mount.Target]; !ok {
			mountsByTarget[mount.Target] = mount
		}
	}

	newMounts := []mounttypes.Mount{}

	toRemove := buildToRemoveSet(flags, flagMountRemove)

	for _, mount := range mountsByTarget {
		if _, exists := toRemove[mount.Target]; !exists {
			newMounts = append(newMounts, mount)
		}
	}
	sort.Sort(byMountSource(newMounts))
	*mounts = newMounts
	return nil
}

func updateGroups(flags *pflag.FlagSet, groups *[]string) error {
	if flags.Changed(flagGroupAdd) {
		values := flags.Lookup(flagGroupAdd).Value.(*opts.ListOpts).GetAll()
		*groups = append(*groups, values...)
	}
	toRemove := buildToRemoveSet(flags, flagGroupRemove)

	newGroups := []string{}
	for _, group := range *groups {
		if _, exists := toRemove[group]; !exists {
			newGroups = append(newGroups, group)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newGroups)

	*groups = newGroups
	return nil
}

func removeDuplicates(entries []string) []string {
	hit := map[string]bool{}
	newEntries := []string{}
	for _, v := range entries {
		if !hit[v] {
			newEntries = append(newEntries, v)
			hit[v] = true
		}
	}
	return newEntries
}

func updateDNSConfig(flags *pflag.FlagSet, config **swarm.DNSConfig) error {
	newConfig := &swarm.DNSConfig{}

	nameservers := (*config).Nameservers
	if flags.Changed(flagDNSAdd) {
		values := flags.Lookup(flagDNSAdd).Value.(*opts.ListOpts).GetAll()
		nameservers = append(nameservers, values...)
	}
	nameservers = removeDuplicates(nameservers)
	toRemove := buildToRemoveSet(flags, flagDNSRemove)
	for _, nameserver := range nameservers {
		if _, exists := toRemove[nameserver]; !exists {
			newConfig.Nameservers = append(newConfig.Nameservers, nameserver)

		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConfig.Nameservers)

	search := (*config).Search
	if flags.Changed(flagDNSSearchAdd) {
		values := flags.Lookup(flagDNSSearchAdd).Value.(*opts.ListOpts).GetAll()
		search = append(search, values...)
	}
	search = removeDuplicates(search)
	toRemove = buildToRemoveSet(flags, flagDNSSearchRemove)
	for _, entry := range search {
		if _, exists := toRemove[entry]; !exists {
			newConfig.Search = append(newConfig.Search, entry)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConfig.Search)

	options := (*config).Options
	if flags.Changed(flagDNSOptionAdd) {
		values := flags.Lookup(flagDNSOptionAdd).Value.(*opts.ListOpts).GetAll()
		options = append(options, values...)
	}
	options = removeDuplicates(options)
	toRemove = buildToRemoveSet(flags, flagDNSOptionRemove)
	for _, option := range options {
		if _, exists := toRemove[option]; !exists {
			newConfig.Options = append(newConfig.Options, option)
		}
	}
	// Sort so that result is predictable.
	sort.Strings(newConfig.Options)

	*config = newConfig
	return nil
}

type byPortConfig []swarm.PortConfig

func (r byPortConfig) Len() int      { return len(r) }
func (r byPortConfig) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byPortConfig) Less(i, j int) bool {
	// We convert PortConfig into `port/protocol`, e.g., `80/tcp`
	// In updatePorts we already filter out with map so there is duplicate entries
	return portConfigToString(&r[i]) < portConfigToString(&r[j])
}

func portConfigToString(portConfig *swarm.PortConfig) string {
	protocol := portConfig.Protocol
	mode := portConfig.PublishMode
	return fmt.Sprintf("%v:%v/%s/%s", portConfig.PublishedPort, portConfig.TargetPort, protocol, mode)
}

// FIXME(vdemeester) port to opts.PortOpt
// This validation is only used for `--publish-rm`.
// The `--publish-rm` takes:
// <TargetPort>[/<Protocol>] (e.g., 80, 80/tcp, 53/udp)
func validatePublishRemove(val string) (string, error) {
	proto, port := nat.SplitProtoPort(val)
	if proto != "tcp" && proto != "udp" {
		return "", fmt.Errorf("invalid protocol '%s' for %s", proto, val)
	}
	if strings.Contains(port, ":") {
		return "", fmt.Errorf("invalid port format: '%s', should be <TargetPort>[/<Protocol>] (e.g., 80, 80/tcp, 53/udp)", port)
	}
	if _, err := nat.ParsePort(port); err != nil {
		return "", err
	}
	return val, nil
}

func updatePorts(flags *pflag.FlagSet, portConfig *[]swarm.PortConfig) error {
	// The key of the map is `port/protocol`, e.g., `80/tcp`
	portSet := map[string]swarm.PortConfig{}

	// Build the current list of portConfig
	for _, entry := range *portConfig {
		if _, ok := portSet[portConfigToString(&entry)]; !ok {
			portSet[portConfigToString(&entry)] = entry
		}
	}

	newPorts := []swarm.PortConfig{}

	// Clean current ports
	toRemove := flags.Lookup(flagPublishRemove).Value.(*opts.PortOpt).Value()
portLoop:
	for _, port := range portSet {
		for _, pConfig := range toRemove {
			if equalProtocol(port.Protocol, pConfig.Protocol) &&
				port.TargetPort == pConfig.TargetPort &&
				equalPublishMode(port.PublishMode, pConfig.PublishMode) {
				continue portLoop
			}
		}

		newPorts = append(newPorts, port)
	}

	// Check to see if there are any conflict in flags.
	if flags.Changed(flagPublishAdd) {
		ports := flags.Lookup(flagPublishAdd).Value.(*opts.PortOpt).Value()

		for _, port := range ports {
			if _, ok := portSet[portConfigToString(&port)]; ok {
				continue
			}
			//portSet[portConfigToString(&port)] = port
			newPorts = append(newPorts, port)
		}
	}

	// Sort the PortConfig to avoid unnecessary updates
	sort.Sort(byPortConfig(newPorts))
	*portConfig = newPorts
	return nil
}

func equalProtocol(prot1, prot2 swarm.PortConfigProtocol) bool {
	return prot1 == prot2 ||
		(prot1 == swarm.PortConfigProtocol("") && prot2 == swarm.PortConfigProtocolTCP) ||
		(prot2 == swarm.PortConfigProtocol("") && prot1 == swarm.PortConfigProtocolTCP)
}

func equalPublishMode(mode1, mode2 swarm.PortConfigPublishMode) bool {
	return mode1 == mode2 ||
		(mode1 == swarm.PortConfigPublishMode("") && mode2 == swarm.PortConfigPublishModeIngress) ||
		(mode2 == swarm.PortConfigPublishMode("") && mode1 == swarm.PortConfigPublishModeIngress)
}

func equalPort(targetPort nat.Port, port swarm.PortConfig) bool {
	return (string(port.Protocol) == targetPort.Proto() &&
		port.TargetPort == uint32(targetPort.Int()))
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

func updateHosts(flags *pflag.FlagSet, hosts *[]string) error {
	// Combine existing Hosts (in swarmkit format) with the host to add (convert to swarmkit format)
	if flags.Changed(flagHostAdd) {
		values := convertExtraHostsToSwarmHosts(flags.Lookup(flagHostAdd).Value.(*opts.ListOpts).GetAll())
		*hosts = append(*hosts, values...)
	}
	// Remove duplicate
	*hosts = removeDuplicates(*hosts)

	keysToRemove := make(map[string]struct{})
	if flags.Changed(flagHostRemove) {
		var empty struct{}
		extraHostsToRemove := flags.Lookup(flagHostRemove).Value.(*opts.ListOpts).GetAll()
		for _, entry := range extraHostsToRemove {
			key := strings.SplitN(entry, ":", 2)[0]
			keysToRemove[key] = empty
		}
	}

	newHosts := []string{}
	for _, entry := range *hosts {
		// Since this is in swarmkit format, we need to find the key, which is canonical_hostname of:
		// IP_address canonical_hostname [aliases...]
		parts := strings.Fields(entry)
		if len(parts) > 1 {
			key := parts[1]
			if _, exists := keysToRemove[key]; !exists {
				newHosts = append(newHosts, entry)
			}
		} else {
			newHosts = append(newHosts, entry)
		}
	}

	// Sort so that result is predictable.
	sort.Strings(newHosts)

	*hosts = newHosts
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

func updateHealthcheck(flags *pflag.FlagSet, containerSpec *swarm.ContainerSpec) error {
	if !anyChanged(flags, flagNoHealthcheck, flagHealthCmd, flagHealthInterval, flagHealthRetries, flagHealthTimeout) {
		return nil
	}
	if containerSpec.Healthcheck == nil {
		containerSpec.Healthcheck = &container.HealthConfig{}
	}
	noHealthcheck, err := flags.GetBool(flagNoHealthcheck)
	if err != nil {
		return err
	}
	if noHealthcheck {
		if !anyChanged(flags, flagHealthCmd, flagHealthInterval, flagHealthRetries, flagHealthTimeout) {
			containerSpec.Healthcheck = &container.HealthConfig{
				Test: []string{"NONE"},
			}
			return nil
		}
		return fmt.Errorf("--%s conflicts with --health-* options", flagNoHealthcheck)
	}
	if len(containerSpec.Healthcheck.Test) > 0 && containerSpec.Healthcheck.Test[0] == "NONE" {
		containerSpec.Healthcheck.Test = nil
	}
	if flags.Changed(flagHealthInterval) {
		val := *flags.Lookup(flagHealthInterval).Value.(*PositiveDurationOpt).Value()
		containerSpec.Healthcheck.Interval = val
	}
	if flags.Changed(flagHealthTimeout) {
		val := *flags.Lookup(flagHealthTimeout).Value.(*PositiveDurationOpt).Value()
		containerSpec.Healthcheck.Timeout = val
	}
	if flags.Changed(flagHealthRetries) {
		containerSpec.Healthcheck.Retries, _ = flags.GetInt(flagHealthRetries)
	}
	if flags.Changed(flagHealthCmd) {
		cmd, _ := flags.GetString(flagHealthCmd)
		if cmd != "" {
			containerSpec.Healthcheck.Test = []string{"CMD-SHELL", cmd}
		} else {
			containerSpec.Healthcheck.Test = nil
		}
	}
	return nil
}
