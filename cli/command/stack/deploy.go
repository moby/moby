package stack

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/aanand/compose-file/loader"
	composetypes "github.com/aanand/compose-file/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	servicecmd "github.com/docker/docker/cli/command/service"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/docker/opts"
	"github.com/docker/go-connections/nat"
)

const (
	defaultNetworkDriver = "overlay"
)

type deployOptions struct {
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
		Tags: map[string]string{"experimental": "", "version": "1.25"},
	}

	flags := cmd.Flags()
	addComposefileFlag(&opts.composefile, flags)
	addRegistryAuthFlag(&opts.sendRegistryAuth, flags)
	return cmd
}

func runDeploy(dockerCli *command.DockerCli, opts deployOptions) error {
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
		fmt.Printf("Ignoring unsupported options: %s\n\n",
			strings.Join(unsupportedProperties, ", "))
	}

	deprecatedProperties := loader.GetDeprecatedProperties(configDetails)
	if len(deprecatedProperties) > 0 {
		fmt.Printf("Ignoring deprecated options:\n\n%s\n\n",
			propertyWarnings(deprecatedProperties))
	}

	ctx := context.Background()
	if err := createNetworks(ctx, dockerCli, config.Networks, opts.namespace); err != nil {
		return err
	}
	return deployServices(ctx, dockerCli, config, opts.namespace, opts.sendRegistryAuth)
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

func createNetworks(
	ctx context.Context,
	dockerCli *command.DockerCli,
	networks map[string]composetypes.NetworkConfig,
	namespace string,
) error {
	client := dockerCli.Client()

	existingNetworks, err := getNetworks(ctx, client, namespace)
	if err != nil {
		return err
	}

	existingNetworkMap := make(map[string]types.NetworkResource)
	for _, network := range existingNetworks {
		existingNetworkMap[network.Name] = network
	}

	for internalName, network := range networks {
		if network.External.Name != "" {
			continue
		}

		name := fmt.Sprintf("%s_%s", namespace, internalName)
		if _, exists := existingNetworkMap[name]; exists {
			continue
		}

		createOpts := types.NetworkCreate{
			// TODO: support network labels from compose file
			Labels:  getStackLabels(namespace, nil),
			Driver:  network.Driver,
			Options: network.DriverOpts,
		}

		if network.Ipam.Driver != "" {
			createOpts.IPAM = &networktypes.IPAM{
				Driver: network.Ipam.Driver,
			}
		}
		// TODO: IPAMConfig.Config

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

func convertNetworks(
	networks map[string]*composetypes.ServiceNetworkConfig,
	namespace string,
	name string,
) []swarm.NetworkAttachmentConfig {
	nets := []swarm.NetworkAttachmentConfig{}
	for networkName, network := range networks {
		nets = append(nets, swarm.NetworkAttachmentConfig{
			// TODO: only do this name mangling in one function
			Target:  namespace + "_" + networkName,
			Aliases: append(network.Aliases, name),
		})
	}
	return nets
}

func convertVolumes(
	serviceVolumes []string,
	stackVolumes map[string]composetypes.VolumeConfig,
	namespace string,
) ([]mount.Mount, error) {
	var mounts []mount.Mount

	for _, volumeString := range serviceVolumes {
		var (
			source, target string
			mountType      mount.Type
			readOnly       bool
			volumeOptions  *mount.VolumeOptions
		)

		// TODO: split Windows path mappings properly
		parts := strings.SplitN(volumeString, ":", 3)

		if len(parts) == 3 {
			source = parts[0]
			target = parts[1]
			if parts[2] == "ro" {
				readOnly = true
			}
		} else if len(parts) == 2 {
			source = parts[0]
			target = parts[1]
		} else if len(parts) == 1 {
			target = parts[0]
		}

		// TODO: catch Windows paths here
		if strings.HasPrefix(source, "/") {
			mountType = mount.TypeBind
		} else {
			mountType = mount.TypeVolume

			stackVolume, exists := stackVolumes[source]
			if !exists {
				// TODO: better error message (include service name)
				return nil, fmt.Errorf("Undefined volume: %s", source)
			}

			if stackVolume.External.Name != "" {
				source = stackVolume.External.Name
			} else {
				volumeOptions = &mount.VolumeOptions{
					Labels: stackVolume.Labels,
				}

				if stackVolume.Driver != "" {
					volumeOptions.DriverConfig = &mount.Driver{
						Name:    stackVolume.Driver,
						Options: stackVolume.DriverOpts,
					}
				}

				// TODO: remove this duplication
				source = fmt.Sprintf("%s_%s", namespace, source)
			}
		}

		mounts = append(mounts, mount.Mount{
			Type:          mountType,
			Source:        source,
			Target:        target,
			ReadOnly:      readOnly,
			VolumeOptions: volumeOptions,
		})
	}

	return mounts, nil
}

func deployServices(
	ctx context.Context,
	dockerCli *command.DockerCli,
	config *composetypes.Config,
	namespace string,
	sendAuth bool,
) error {
	apiClient := dockerCli.Client()
	out := dockerCli.Out()
	services := config.Services
	volumes := config.Volumes

	existingServices, err := getServices(ctx, apiClient, namespace)
	if err != nil {
		return err
	}

	existingServiceMap := make(map[string]swarm.Service)
	for _, service := range existingServices {
		existingServiceMap[service.Spec.Name] = service
	}

	for _, service := range services {
		name := fmt.Sprintf("%s_%s", namespace, service.Name)

		serviceSpec, err := convertService(namespace, service, volumes)
		if err != nil {
			return err
		}

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
			if err := apiClient.ServiceUpdate(
				ctx,
				service.ID,
				service.Version,
				serviceSpec,
				updateOpts,
			); err != nil {
				return err
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

func convertService(
	namespace string,
	service composetypes.ServiceConfig,
	volumes map[string]composetypes.VolumeConfig,
) (swarm.ServiceSpec, error) {
	// TODO: remove this duplication
	name := fmt.Sprintf("%s_%s", namespace, service.Name)

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

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: getStackLabels(namespace, service.Deploy.Labels),
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:           service.Image,
				Command:         service.Entrypoint,
				Args:            service.Command,
				Hostname:        service.Hostname,
				Env:             convertEnvironment(service.Environment),
				Labels:          getStackLabels(namespace, service.Labels),
				Dir:             service.WorkingDir,
				User:            service.User,
				Mounts:          mounts,
				StopGracePeriod: service.StopGracePeriod,
			},
			Resources:     resources,
			RestartPolicy: restartPolicy,
			Placement: &swarm.Placement{
				Constraints: service.Deploy.Placement.Constraints,
			},
		},
		EndpointSpec: endpoint,
		Mode:         mode,
		Networks:     convertNetworks(service.Networks, namespace, service.Name),
		UpdateConfig: convertUpdateConfig(service.Deploy.UpdateConfig),
	}

	return serviceSpec, nil
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
		case policy.IsNone(), policy.IsAlways(), policy.IsUnlessStopped():
			return nil, nil
		case policy.IsOnFailure():
			attempts := uint64(policy.MaximumRetryCount)
			return &swarm.RestartPolicy{
				Condition:   swarm.RestartPolicyConditionOnFailure,
				MaxAttempts: &attempts,
			}, nil
		}
	}
	attempts := uint64(*source.MaxAttempts)
	return &swarm.RestartPolicy{
		Condition:   swarm.RestartPolicyCondition(source.Condition),
		Delay:       source.Delay,
		MaxAttempts: &attempts,
		Window:      source.Window,
	}, nil
}

func convertUpdateConfig(source *composetypes.UpdateConfig) *swarm.UpdateConfig {
	if source == nil {
		return nil
	}
	return &swarm.UpdateConfig{
		Parallelism:     source.Parallelism,
		Delay:           source.Delay,
		FailureAction:   source.FailureAction,
		Monitor:         source.Monitor,
		MaxFailureRatio: source.MaxFailureRatio,
	}
}

func convertResources(source composetypes.Resources) (*swarm.ResourceRequirements, error) {
	resources := &swarm.ResourceRequirements{}
	if source.Limits != nil {
		cpus, err := opts.ParseCPUs(source.Limits.NanoCPUs)
		if err != nil {
			return nil, err
		}
		resources.Limits = &swarm.Resources{
			NanoCPUs:    cpus,
			MemoryBytes: int64(source.Limits.MemoryBytes),
		}
	}
	if source.Reservations != nil {
		cpus, err := opts.ParseCPUs(source.Reservations.NanoCPUs)
		if err != nil {
			return nil, err
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
			servicecmd.ConvertPortToPortConfig(port, portBindings)...)
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
	case "replicated":
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: replicas}
	default:
		return serviceMode, fmt.Errorf("Unknown mode: %s", mode)
	}
	return serviceMode, nil
}
