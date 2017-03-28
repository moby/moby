package stack

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/compose/convert"
	"github.com/docker/docker/cli/compose/loader"
	composetypes "github.com/docker/docker/cli/compose/types"
	apiclient "github.com/docker/docker/client"
	dockerclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func deployCompose(ctx context.Context, dockerCli *command.DockerCli, opts deployOptions) error {
	configDetails, err := getConfigDetails(opts)
	if err != nil {
		return err
	}

	config, err := loader.Load(configDetails)
	if err != nil {
		if fpe, ok := err.(*loader.ForbiddenPropertiesError); ok {
			return errors.Errorf("Compose file contains unsupported options:\n\n%s\n",
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

	namespace := convert.NewNamespace(opts.namespace)

	if opts.prune {
		services := map[string]struct{}{}
		for _, service := range config.Services {
			services[service.Name] = struct{}{}
		}
		pruneServices(ctx, dockerCli, namespace, services)
	}

	serviceNetworks := getServicesDeclaredNetworks(config.Services)
	networks, externalNetworks := convert.Networks(namespace, config.Networks, serviceNetworks)
	if err := validateExternalNetworks(ctx, dockerCli, externalNetworks); err != nil {
		return err
	}
	if err := createNetworks(ctx, dockerCli, namespace, networks); err != nil {
		return err
	}

	secrets, err := convert.Secrets(namespace, config.Secrets)
	if err != nil {
		return err
	}
	if err := createSecrets(ctx, dockerCli, namespace, secrets); err != nil {
		return err
	}

	services, err := convert.Services(namespace, config, dockerCli.Client())
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
	details.Environment, err = buildEnvironment(os.Environ())
	if err != nil {
		return details, err
	}
	return details, nil
}

func buildEnvironment(env []string) (map[string]string, error) {
	result := make(map[string]string, len(env))
	for _, s := range env {
		// if value is empty, s is like "K=", not "K".
		if !strings.Contains(s, "=") {
			return result, errors.Errorf("unexpected environment %q", s)
		}
		kv := strings.SplitN(s, "=", 2)
		result[kv[0]] = kv[1]
	}
	return result, nil
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

func validateExternalNetworks(
	ctx context.Context,
	dockerCli *command.DockerCli,
	externalNetworks []string) error {
	client := dockerCli.Client()

	for _, networkName := range externalNetworks {
		network, err := client.NetworkInspect(ctx, networkName, false)
		if err != nil {
			if dockerclient.IsErrNetworkNotFound(err) {
				return errors.Errorf("network %q is declared as external, but could not be found. You need to create the network before the stack is deployed (with overlay driver)", networkName)
			}
			return err
		}
		if network.Scope != "swarm" {
			return errors.Errorf("network %q is declared as external, but it is not in the right scope: %q instead of %q", networkName, network.Scope, "swarm")
		}
	}

	return nil
}

func createSecrets(
	ctx context.Context,
	dockerCli *command.DockerCli,
	namespace convert.Namespace,
	secrets []swarm.SecretSpec,
) error {
	client := dockerCli.Client()

	for _, secretSpec := range secrets {
		secret, _, err := client.SecretInspectWithRaw(ctx, secretSpec.Name)
		if err == nil {
			// secret already exists, then we update that
			if err := client.SecretUpdate(ctx, secret.ID, secret.Meta.Version, secretSpec); err != nil {
				return err
			}
		} else if apiclient.IsErrSecretNotFound(err) {
			// secret does not exist, then we create a new one.
			if _, err := client.SecretCreate(ctx, secretSpec); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func createNetworks(
	ctx context.Context,
	dockerCli *command.DockerCli,
	namespace convert.Namespace,
	networks map[string]types.NetworkCreate,
) error {
	client := dockerCli.Client()

	existingNetworks, err := getStackNetworks(ctx, client, namespace.Name())
	if err != nil {
		return err
	}

	existingNetworkMap := make(map[string]types.NetworkResource)
	for _, network := range existingNetworks {
		existingNetworkMap[network.Name] = network
	}

	for internalName, createOpts := range networks {
		name := namespace.Scope(internalName)
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

func deployServices(
	ctx context.Context,
	dockerCli *command.DockerCli,
	services map[string]swarm.ServiceSpec,
	namespace convert.Namespace,
	sendAuth bool,
) error {
	apiClient := dockerCli.Client()
	out := dockerCli.Out()

	existingServices, err := getServices(ctx, apiClient, namespace.Name())
	if err != nil {
		return err
	}

	existingServiceMap := make(map[string]swarm.Service)
	for _, service := range existingServices {
		existingServiceMap[service.Spec.Name] = service
	}

	for internalName, serviceSpec := range services {
		name := namespace.Scope(internalName)

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
