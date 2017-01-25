package stack

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aanand/compose-file/loader"
	composetypes "github.com/aanand/compose-file/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

const (
	defaultNetworkDriver = "overlay"
)

type deployOptions struct {
	bundlefile       string
	composefile      string
	namespace        string
	sendRegistryAuth bool
}

func newDeployCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts deployOptions

	cmd := &cobra.Command{
		Use:     "deploy [OPTIONS] STACK",
		Aliases: []string{"up"},
		Short:   "Deploy a new stack or update an existing stack",
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.namespace = args[0]
			return runDeploy(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	addBundlefileFlag(&opts.bundlefile, flags)
	addComposefileFlag(&opts.composefile, flags)
	addRegistryAuthFlag(&opts.sendRegistryAuth, flags)
	return cmd
}

func runDeploy(dockerCli *command.DockerCli, opts deployOptions) error {
	ctx := context.Background()

	switch {
	case opts.bundlefile == "" && opts.composefile == "":
		return fmt.Errorf("Please specify either a bundle file (with --bundle-file) or a Compose file (with --compose-file).")
	case opts.bundlefile != "" && opts.composefile != "":
		return fmt.Errorf("You cannot specify both a bundle file and a Compose file.")
	case opts.bundlefile != "":
		return deployBundle(ctx, dockerCli, opts)
	default:
		return deployCompose(ctx, dockerCli, opts)
	}
}

// checkDaemonIsSwarmManager does an Info API call to verify that the daemon is
// a swarm manager. This is necessary because we must create networks before we
// create services, but the API call for creating a network does not return a
// proper status code when it can't create a network in the "global" scope.
func checkDaemonIsSwarmManager(ctx context.Context, dockerCli *command.DockerCli) error {
	info, err := dockerCli.Client().Info(ctx)
	if err != nil {
		return err
	}
	if !info.Swarm.ControlAvailable {
		return errors.New("This node is not a swarm manager. Use \"docker swarm init\" or \"docker swarm join\" to connect this node to swarm and try again.")
	}
	return nil
}

func deployCompose(ctx context.Context, dockerCli *command.DockerCli, opts deployOptions) error {
	configDetails, err := getConfigDetails(opts)
	if err != nil {
		return err
	}

	config, err := loader.Load(configDetails)
	if err != nil {
		if fpe, ok := err.(*loader.ForbiddenPropertiesError); ok {
			return fmt.Errorf("Compose file contains unsupported options:\n\n%s\n",
				propertyWarnings(fpe.Properties))
		}

		return err
	}

	unsupportedProperties := loader.GetUnsupportedProperties(configDetails)
	if len(unsupportedProperties) > 0 {
		fmt.Fprintf(dockerCli.Err(), "Ignoring unsupported options: %s\n\n",
			strings.Join(unsupportedProperties, ", "))
	}

	deprecatedProperties := loader.GetDeprecatedProperties(configDetails)
	if len(deprecatedProperties) > 0 {
		fmt.Fprintf(dockerCli.Err(), "Ignoring deprecated options:\n\n%s\n\n",
			propertyWarnings(deprecatedProperties))
	}

	if err := checkDaemonIsSwarmManager(ctx, dockerCli); err != nil {
		return err
	}

	namespace := namespace{name: opts.namespace}

	serviceNetworks := getServicesDeclaredNetworks(config.Services)
	networks, externalNetworks := convertNetworks(namespace, config.Networks, serviceNetworks)
	if err := validateExternalNetworks(ctx, dockerCli, externalNetworks); err != nil {
		return err
	}
	if err := createNetworks(ctx, dockerCli, namespace, networks); err != nil {
		return err
	}
	services, err := convertServices(namespace, config)
	if err != nil {
		return err
	}
	return deployServices(ctx, dockerCli, services, namespace, opts.sendRegistryAuth)
}
func getServicesDeclaredNetworks(serviceConfigs []composetypes.ServiceConfig) map[string]struct{} {
	serviceNetworks := map[string]struct{}{}
	for _, serviceConfig := range serviceConfigs {
		if len(serviceConfig.Networks) == 0 {
			serviceNetworks["default"] = struct{}{}
			continue
		}
		for network := range serviceConfig.Networks {
			serviceNetworks[network] = struct{}{}
		}
	}
	return serviceNetworks
}

func propertyWarnings(properties map[string]string) string {
	var msgs []string
	for name, description := range properties {
		msgs = append(msgs, fmt.Sprintf("%s: %s", name, description))
	}
	sort.Strings(msgs)
	return strings.Join(msgs, "\n\n")
}

func getConfigDetails(opts deployOptions) (composetypes.ConfigDetails, error) {
	var details composetypes.ConfigDetails
	var err error

	details.WorkingDir, err = os.Getwd()
	if err != nil {
		return details, err
	}

	configFile, err := getConfigFile(opts.composefile)
	if err != nil {
		return details, err
	}
	// TODO: support multiple files
	details.ConfigFiles = []composetypes.ConfigFile{*configFile}
	return details, nil
}

func getConfigFile(filename string) (*composetypes.ConfigFile, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config, err := loader.ParseYAML(bytes)
	if err != nil {
		return nil, err
	}
	return &composetypes.ConfigFile{
		Filename: filename,
		Config:   config,
	}, nil
}

func convertNetworks(
	namespace namespace,
	networks map[string]composetypes.NetworkConfig,
	servicesNetworks map[string]struct{},
) (map[string]types.NetworkCreate, []string) {
	if networks == nil {
		networks = make(map[string]composetypes.NetworkConfig)
	}

	externalNetworks := []string{}
	result := make(map[string]types.NetworkCreate)

	for internalName := range servicesNetworks {
		network := networks[internalName]
		if network.External.External {
			externalNetworks = append(externalNetworks, network.External.Name)
			continue
		}

		createOpts := types.NetworkCreate{
			Labels:  getStackLabels(namespace.name, network.Labels),
			Driver:  network.Driver,
			Options: network.DriverOpts,
		}

		if network.Ipam.Driver != "" || len(network.Ipam.Config) > 0 {
			createOpts.IPAM = &networktypes.IPAM{}
		}

		if network.Ipam.Driver != "" {
			createOpts.IPAM.Driver = network.Ipam.Driver
		}
		for _, ipamConfig := range network.Ipam.Config {
			config := networktypes.IPAMConfig{
				Subnet: ipamConfig.Subnet,
			}
			createOpts.IPAM.Config = append(createOpts.IPAM.Config, config)
		}
		result[internalName] = createOpts
	}

	return result, externalNetworks
}

func validateExternalNetworks(
	ctx context.Context,
	dockerCli *command.DockerCli,
	externalNetworks []string) error {
	client := dockerCli.Client()

	for _, networkName := range externalNetworks {
		network, err := client.NetworkInspect(ctx, networkName)
		if err != nil {
			if dockerclient.IsErrNetworkNotFound(err) {
				return fmt.Errorf("network %q is declared as external, but could not be found. You need to create the network before the stack is deployed (with overlay driver)", networkName)
			}
			return err
		}
		if network.Scope != "swarm" {
			return fmt.Errorf("network %q is declared as external, but it is not in the right scope: %q instead of %q", networkName, network.Scope, "swarm")
		}
	}

	return nil
}

func createNetworks(
	ctx context.Context,
	dockerCli *command.DockerCli,
	namespace namespace,
	networks map[string]types.NetworkCreate,
) error {
	client := dockerCli.Client()

	existingNetworks, err := getStackNetworks(ctx, client, namespace.name)
	if err != nil {
		return err
	}

	existingNetworkMap := make(map[string]types.NetworkResource)
	for _, network := range existingNetworks {
		existingNetworkMap[network.Name] = network
	}

	for internalName, createOpts := range networks {
		name := namespace.scope(internalName)
		if _, exists := existingNetworkMap[name]; exists {
			continue
		}

		if createOpts.Driver == "" {
			createOpts.Driver = defaultNetworkDriver
		}

		fmt.Fprintf(dockerCli.Out(), "Creating network %s\n", name)
		if _, err := client.NetworkCreate(ctx, name, createOpts); err != nil {
			return err
		}
	}

	return nil
}

func convertServiceNetworks(
	networks map[string]*composetypes.ServiceNetworkConfig,
	networkConfigs map[string]composetypes.NetworkConfig,
	namespace namespace,
	name string,
) ([]swarm.NetworkAttachmentConfig, error) {
	if len(networks) == 0 {
		return []swarm.NetworkAttachmentConfig{
			{
				Target:  namespace.scope("default"),
				Aliases: []string{name},
			},
		}, nil
	}

	nets := []swarm.NetworkAttachmentConfig{}
	for networkName, network := range networks {
		networkConfig, ok := networkConfigs[networkName]
		if !ok {
			return []swarm.NetworkAttachmentConfig{}, fmt.Errorf("invalid network: %s", networkName)
		}
		var aliases []string
		if network != nil {
			aliases = network.Aliases
		}
		target := namespace.scope(networkName)
		if networkConfig.External.External {
			target = networkConfig.External.Name
		}
		nets = append(nets, swarm.NetworkAttachmentConfig{
			Target:  target,
			Aliases: append(aliases, name),
		})
	}
	return nets, nil
}

func convertVolumes(
	serviceVolumes []string,
	stackVolumes map[string]composetypes.VolumeConfig,
	namespace namespace,
) ([]mount.Mount, error) {
	var mounts []mount.Mount

	for _, volumeSpec := range serviceVolumes {
		mount, err := convertVolumeToMount(volumeSpec, stackVolumes, namespace)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

func convertVolumeToMount(
	volumeSpec string,
	stackVolumes map[string]composetypes.VolumeConfig,
	namespace namespace,
) (mount.Mount, error) {
	var source, target string
	var mode []string

	// TODO: split Windows path mappings properly
	parts := strings.SplitN(volumeSpec, ":", 3)

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return mount.Mount{}, fmt.Errorf("invalid volume: %s", volumeSpec)
		}
	}

	switch len(parts) {
	case 3:
		source = parts[0]
		target = parts[1]
		mode = strings.Split(parts[2], ",")
	case 2:
		source = parts[0]
		target = parts[1]
	case 1:
		target = parts[0]
	}

	if source == "" {
		// Anonymous volume
		return mount.Mount{
			Type:   mount.TypeVolume,
			Target: target,
		}, nil
	}

	// TODO: catch Windows paths here
	if strings.HasPrefix(source, "/") {
		return mount.Mount{
			Type:        mount.TypeBind,
			Source:      source,
			Target:      target,
			ReadOnly:    isReadOnly(mode),
			BindOptions: getBindOptions(mode),
		}, nil
	}

	stackVolume, exists := stackVolumes[source]
	if !exists {
		return mount.Mount{}, fmt.Errorf("undefined volume: %s", source)
	}

	var volumeOptions *mount.VolumeOptions
	if stackVolume.External.Name != "" {
		source = stackVolume.External.Name
	} else {
		volumeOptions = &mount.VolumeOptions{
			Labels: getStackLabels(namespace.name, stackVolume.Labels),
			NoCopy: isNoCopy(mode),
		}

		if stackVolume.Driver != "" {
			volumeOptions.DriverConfig = &mount.Driver{
				Name:    stackVolume.Driver,
				Options: stackVolume.DriverOpts,
			}
		}
		source = namespace.scope(source)
	}
	return mount.Mount{
		Type:          mount.TypeVolume,
		Source:        source,
		Target:        target,
		ReadOnly:      isReadOnly(mode),
		VolumeOptions: volumeOptions,
	}, nil
}

func modeHas(mode []string, field string) bool {
	for _, item := range mode {
		if item == field {
			return true
		}
	}
	return false
}

func isReadOnly(mode []string) bool {
	return modeHas(mode, "ro")
}

func isNoCopy(mode []string) bool {
	return modeHas(mode, "nocopy")
}

func getBindOptions(mode []string) *mount.BindOptions {
	for _, item := range mode {
		if strings.Contains(item, "private") || strings.Contains(item, "shared") || strings.Contains(item, "slave") {
			return &mount.BindOptions{Propagation: mount.Propagation(item)}
		}
	}
	return nil
}

func deployServices(
	ctx context.Context,
	dockerCli *command.DockerCli,
	services map[string]swarm.ServiceSpec,
	namespace namespace,
	sendAuth bool,
) error {
	apiClient := dockerCli.Client()
	out := dockerCli.Out()

	existingServices, err := getServices(ctx, apiClient, namespace.name)
	if err != nil {
		return err
	}

	existingServiceMap := make(map[string]swarm.Service)
	for _, service := range existingServices {
		existingServiceMap[service.Spec.Name] = service
	}

	for internalName, serviceSpec := range services {
		name := namespace.scope(internalName)

		encodedAuth := ""
		if sendAuth {
			// Retrieve encoded auth token from the image reference
			image := serviceSpec.TaskTemplate.ContainerSpec.Image
			encodedAuth, err = command.RetrieveAuthTokenFromImage(ctx, dockerCli, image)
			if err != nil {
				return err
			}
		}

		if service, exists := existingServiceMap[name]; exists {
			fmt.Fprintf(out, "Updating service %s (id: %s)\n", name, service.ID)

			updateOpts := types.ServiceUpdateOptions{}
			if sendAuth {
				updateOpts.EncodedRegistryAuth = encodedAuth
			}
			response, err := apiClient.ServiceUpdate(
				ctx,
				service.ID,
				service.Version,
				serviceSpec,
				updateOpts,
			)
			if err != nil {
				return err
			}

			for _, warning := range response.Warnings {
				fmt.Fprintln(dockerCli.Err(), warning)
			}
		} else {
			fmt.Fprintf(out, "Creating service %s\n", name)

			createOpts := types.ServiceCreateOptions{}
			if sendAuth {
				createOpts.EncodedRegistryAuth = encodedAuth
			}
			if _, err := apiClient.ServiceCreate(ctx, serviceSpec, createOpts); err != nil {
				return err
			}
		}
	}

	return nil
}

func convertServices(
	namespace namespace,
	config *composetypes.Config,
) (map[string]swarm.ServiceSpec, error) {
	result := make(map[string]swarm.ServiceSpec)

	services := config.Services
	volumes := config.Volumes
	networks := config.Networks

	for _, service := range services {
		serviceSpec, err := convertService(namespace, service, networks, volumes)
		if err != nil {
			return nil, err
		}
		result[service.Name] = serviceSpec
	}

	return result, nil
}

func convertService(
	namespace namespace,
	service composetypes.ServiceConfig,
	networkConfigs map[string]composetypes.NetworkConfig,
	volumes map[string]composetypes.VolumeConfig,
) (swarm.ServiceSpec, error) {
	name := namespace.scope(service.Name)

	endpoint, err := convertEndpointSpec(service.Ports)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	mode, err := convertDeployMode(service.Deploy.Mode, service.Deploy.Replicas)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	mounts, err := convertVolumes(service.Volumes, volumes, namespace)
	if err != nil {
		// TODO: better error message (include service name)
		return swarm.ServiceSpec{}, err
	}

	resources, err := convertResources(service.Deploy.Resources)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	restartPolicy, err := convertRestartPolicy(
		service.Restart, service.Deploy.RestartPolicy)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	healthcheck, err := convertHealthcheck(service.HealthCheck)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	networks, err := convertServiceNetworks(service.Networks, networkConfigs, namespace, service.Name)
	if err != nil {
		return swarm.ServiceSpec{}, err
	}

	var logDriver *swarm.Driver
	if service.Logging != nil {
		logDriver = &swarm.Driver{
			Name:    service.Logging.Driver,
			Options: service.Logging.Options,
		}
	}

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: getStackLabels(namespace.name, service.Deploy.Labels),
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:           service.Image,
				Command:         service.Entrypoint,
				Args:            service.Command,
				Hostname:        service.Hostname,
				Hosts:           convertExtraHosts(service.ExtraHosts),
				Healthcheck:     healthcheck,
				DNSConfig:       convertDNS(service.Dns, service.DnsSearch),
				Env:             convertEnvironment(service.Environment),
				Labels:          getStackLabels(namespace.name, service.Labels),
				Dir:             service.WorkingDir,
				User:            service.User,
				Mounts:          mounts,
				StopGracePeriod: service.StopGracePeriod,
				TTY:             service.Tty,
				OpenStdin:       service.StdinOpen,
			},
			LogDriver:     logDriver,
			Resources:     resources,
			RestartPolicy: restartPolicy,
			Placement: &swarm.Placement{
				Constraints: service.Deploy.Placement.Constraints,
			},
		},
		EndpointSpec: endpoint,
		Mode:         mode,
		Networks:     networks,
		UpdateConfig: convertUpdateConfig(service.Deploy.UpdateConfig),
	}

	return serviceSpec, nil
}
func convertDNS(dns []string, dnsSearch []string) *swarm.DNSConfig {
	return &swarm.DNSConfig{
		Nameservers: dns,
		Search:      dnsSearch,
	}
}

func convertExtraHosts(extraHosts map[string]string) []string {
	hosts := []string{}
	for host, ip := range extraHosts {
		hosts = append(hosts, fmt.Sprintf("%s %s", ip, host))
	}
	return hosts
}

func convertHealthcheck(healthcheck *composetypes.HealthCheckConfig) (*container.HealthConfig, error) {
	if healthcheck == nil {
		return nil, nil
	}
	var (
		err               error
		timeout, interval time.Duration
		retries           int
	)
	if healthcheck.Disable {
		if len(healthcheck.Test) != 0 {
			return nil, fmt.Errorf("command and disable key can't be set at the same time")
		}
		return &container.HealthConfig{
			Test: []string{"NONE"},
		}, nil

	}
	if healthcheck.Timeout != "" {
		timeout, err = time.ParseDuration(healthcheck.Timeout)
		if err != nil {
			return nil, err
		}
	}
	if healthcheck.Interval != "" {
		interval, err = time.ParseDuration(healthcheck.Interval)
		if err != nil {
			return nil, err
		}
	}
	if healthcheck.Retries != nil {
		retries = int(*healthcheck.Retries)
	}
	return &container.HealthConfig{
		Test:     healthcheck.Test,
		Timeout:  timeout,
		Interval: interval,
		Retries:  retries,
	}, nil
}

func convertRestartPolicy(restart string, source *composetypes.RestartPolicy) (*swarm.RestartPolicy, error) {
	// TODO: log if restart is being ignored
	if source == nil {
		policy, err := runconfigopts.ParseRestartPolicy(restart)
		if err != nil {
			return nil, err
		}
		// TODO: is this an accurate convertion?
		switch {
		case policy.IsNone():
			return nil, nil
		case policy.IsAlways(), policy.IsUnlessStopped():
			return &swarm.RestartPolicy{
				Condition: swarm.RestartPolicyConditionAny,
			}, nil
		case policy.IsOnFailure():
			attempts := uint64(policy.MaximumRetryCount)
			return &swarm.RestartPolicy{
				Condition:   swarm.RestartPolicyConditionOnFailure,
				MaxAttempts: &attempts,
			}, nil
		}
	}
	return &swarm.RestartPolicy{
		Condition:   swarm.RestartPolicyCondition(source.Condition),
		Delay:       source.Delay,
		MaxAttempts: source.MaxAttempts,
		Window:      source.Window,
	}, nil
}

func convertUpdateConfig(source *composetypes.UpdateConfig) *swarm.UpdateConfig {
	if source == nil {
		return nil
	}
	parallel := uint64(1)
	if source.Parallelism != nil {
		parallel = *source.Parallelism
	}
	return &swarm.UpdateConfig{
		Parallelism:     parallel,
		Delay:           source.Delay,
		FailureAction:   source.FailureAction,
		Monitor:         source.Monitor,
		MaxFailureRatio: source.MaxFailureRatio,
	}
}

func convertResources(source composetypes.Resources) (*swarm.ResourceRequirements, error) {
	resources := &swarm.ResourceRequirements{}
	var err error
	if source.Limits != nil {
		var cpus int64
		if source.Limits.NanoCPUs != "" {
			cpus, err = opts.ParseCPUs(source.Limits.NanoCPUs)
			if err != nil {
				return nil, err
			}
		}
		resources.Limits = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Limits.MemoryBytes),
		}
	}
	if source.Reservations != nil {
		var cpus int64
		if source.Reservations.NanoCPUs != "" {
			cpus, err = opts.ParseCPUs(source.Reservations.NanoCPUs)
			if err != nil {
				return nil, err
			}
		}
		resources.Reservations = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Reservations.MemoryBytes),
		}
	}
	return resources, nil
}

func convertEndpointSpec(source []string) (*swarm.EndpointSpec, error) {
	portConfigs := []swarm.PortConfig{}
	ports, portBindings, err := nat.ParsePortSpecs(source)
	if err != nil {
		return nil, err
	}

	for port := range ports {
		portConfigs = append(
			portConfigs,
			opts.ConvertPortToPortConfig(port, portBindings)...)
	}

	return &swarm.EndpointSpec{Ports: portConfigs}, nil
}

func convertEnvironment(source map[string]string) []string {
	var output []string

	for name, value := range source {
		output = append(output, fmt.Sprintf("%s=%s", name, value))
	}

	return output
}

func convertDeployMode(mode string, replicas *uint64) (swarm.ServiceMode, error) {
	serviceMode := swarm.ServiceMode{}

	switch mode {
	case "global":
		if replicas != nil {
			return serviceMode, fmt.Errorf("replicas can only be used with replicated mode")
		}
		serviceMode.Global = &swarm.GlobalService{}
	case "replicated", "":
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: replicas}
	default:
		return serviceMode, fmt.Errorf("Unknown mode: %s", mode)
	}
	return serviceMode, nil
}
