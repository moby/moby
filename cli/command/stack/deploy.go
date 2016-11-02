package stack

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/aanand/compose-file/loader"
	composetypes "github.com/aanand/compose-file/types"
	"github.com/docker/docker/api/types"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	servicecmd "github.com/docker/docker/cli/command/service"
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
		return err
	}

	ctx := context.Background()
	if err := createNetworks(ctx, dockerCli, config.Networks, opts.namespace); err != nil {
		return err
	}
	return deployServices(ctx, dockerCli, config, opts.namespace, opts.sendRegistryAuth)
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
	return loader.ParseYAML(bytes, filename)
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
		if network.ExternalName != "" {
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

	serviceSpec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: getStackLabels(namespace, service.Labels),
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image:   service.Image,
				Command: service.Entrypoint,
				Args:    service.Command,
				Env:     convertEnvironment(service.Environment),
				Labels:  getStackLabels(namespace, service.Deploy.Labels),
				Dir:     service.WorkingDir,
				User:    service.User,
			},
			Placement: &swarm.Placement{
				Constraints: service.Deploy.Placement.Constraints,
			},
		},
		EndpointSpec: endpoint,
		Mode:         mode,
		Networks:     convertNetworks(service.Networks, namespace, service.Name),
	}

	if service.StopGracePeriod != nil {
		stopGrace, err := time.ParseDuration(*service.StopGracePeriod)
		if err != nil {
			return swarm.ServiceSpec{}, err
		}
		serviceSpec.TaskTemplate.ContainerSpec.StopGracePeriod = &stopGrace
	}

	// TODO: convert mounts
	return serviceSpec, nil
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

func convertDeployMode(mode string, replicas uint64) (swarm.ServiceMode, error) {
	serviceMode := swarm.ServiceMode{}

	switch mode {
	case "global":
		if replicas != 0 {
			return serviceMode, fmt.Errorf("replicas can only be used with replicated mode")
		}
		serviceMode.Global = &swarm.GlobalService{}
	case "replicated":
		serviceMode.Replicated = &swarm.ReplicatedService{Replicas: &replicas}
	default:
		return serviceMode, fmt.Errorf("Unknown mode: %s", mode)
	}
	return serviceMode, nil
}
