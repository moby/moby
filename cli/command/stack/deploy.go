package stack

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/compose/convert"
	"github.com/docker/docker/cli/compose/loader"
	composetypes "github.com/docker/docker/cli/compose/types"
	dockerclient "github.com/docker/docker/client"
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

	namespace := convert.NewNamespace(opts.namespace)

	networks, externalNetworks := convert.Networks(namespace, config.Networks)
	if err := validateExternalNetworks(ctx, dockerCli, externalNetworks); err != nil {
		return err
	}
	if err := createNetworks(ctx, dockerCli, namespace, networks); err != nil {
		return err
	}
	services, err := convert.Services(namespace, config)
	if err != nil {
		return err
	}
	return deployServices(ctx, dockerCli, services, namespace, opts.sendRegistryAuth)
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
