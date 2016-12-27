package container

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/net/context"

	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/image"
	"github.com/docker/docker/pkg/jsonmessage"
	// FIXME migrate to docker/distribution/reference
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	apiclient "github.com/docker/docker/client"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type createOptions struct {
	name string
}

// NewCreateCommand creates a new cobra.Command for `docker create`
func NewCreateCommand(dockerCli *command.DockerCli) *cobra.Command {
	var opts createOptions
	var copts *runconfigopts.ContainerOptions

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Create a new container",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			copts.Image = args[0]
			if len(args) > 1 {
				copts.Args = args[1:]
			}
			return runCreate(dockerCli, cmd.Flags(), &opts, copts)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.StringVar(&opts.name, "name", "", "Assign a name to the container")

	// Add an explicit help that doesn't have a `-h` to prevent the conflict
	// with hostname
	flags.Bool("help", false, "Print usage")

	command.AddTrustedFlags(flags, true)
	copts = runconfigopts.AddFlags(flags)
	return cmd
}

func runCreate(dockerCli *command.DockerCli, flags *pflag.FlagSet, opts *createOptions, copts *runconfigopts.ContainerOptions) error {
	config, hostConfig, networkingConfig, err := runconfigopts.Parse(flags, copts)
	if err != nil {
		reportError(dockerCli.Err(), "create", err.Error(), true)
		return cli.StatusError{StatusCode: 125}
	}
	response, err := createContainer(context.Background(), dockerCli, config, hostConfig, networkingConfig, hostConfig.ContainerIDFile, opts.name)
	if err != nil {
		return err
	}
	fmt.Fprintf(dockerCli.Out(), "%s\n", response.ID)
	return nil
}

func pullImage(ctx context.Context, dockerCli *command.DockerCli, image string, out io.Writer) error {
	ref, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)
	encodedAuth, err := command.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImageCreateOptions{
		RegistryAuth: encodedAuth,
	}

	responseBody, err := dockerCli.Client().ImageCreate(ctx, image, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(
		responseBody,
		out,
		dockerCli.Out().FD(),
		dockerCli.Out().IsTerminal(),
		nil)
}

type cidFile struct {
	path    string
	file    *os.File
	written bool
}

func (cid *cidFile) Close() error {
	cid.file.Close()

	if !cid.written {
		if err := os.Remove(cid.path); err != nil {
			return fmt.Errorf("failed to remove the CID file '%s': %s \n", cid.path, err)
		}
	}

	return nil
}

func (cid *cidFile) Write(id string) error {
	if _, err := cid.file.Write([]byte(id)); err != nil {
		return fmt.Errorf("Failed to write the container ID to the file: %s", err)
	}
	cid.written = true
	return nil
}

func newCIDFile(path string) (*cidFile, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("Container ID file found, make sure the other container isn't running or delete %s", path)
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the container ID file: %s", err)
	}

	return &cidFile{path: path, file: f}, nil
}

func createContainer(ctx context.Context, dockerCli *command.DockerCli, config *container.Config, hostConfig *container.HostConfig, networkingConfig *networktypes.NetworkingConfig, cidfile, name string) (*container.ContainerCreateCreatedBody, error) {
	stderr := dockerCli.Err()

	var containerIDFile *cidFile
	if cidfile != "" {
		var err error
		if containerIDFile, err = newCIDFile(cidfile); err != nil {
			return nil, err
		}
		defer containerIDFile.Close()
	}

	var trustedRef reference.Canonical
	_, ref, err := reference.ParseIDOrReference(config.Image)
	if err != nil {
		return nil, err
	}
	if ref != nil {
		ref = reference.WithDefaultTag(ref)

		if ref, ok := ref.(reference.NamedTagged); ok && command.IsTrusted() {
			var err error
			trustedRef, err = image.TrustedReference(ctx, dockerCli, ref, nil)
			if err != nil {
				return nil, err
			}
			config.Image = trustedRef.String()
		}
	}

	//create the container
	response, err := dockerCli.Client().ContainerCreate(ctx, config, hostConfig, networkingConfig, name)

	//if image not found try to pull it
	if err != nil {
		if apiclient.IsErrImageNotFound(err) && ref != nil {
			fmt.Fprintf(stderr, "Unable to find image '%s' locally\n", ref.String())

			// we don't want to write to stdout anything apart from container.ID
			if err = pullImage(ctx, dockerCli, config.Image, stderr); err != nil {
				return nil, err
			}
			if ref, ok := ref.(reference.NamedTagged); ok && trustedRef != nil {
				if err := image.TagTrusted(ctx, dockerCli, trustedRef, ref); err != nil {
					return nil, err
				}
			}
			// Retry
			var retryErr error
			response, retryErr = dockerCli.Client().ContainerCreate(ctx, config, hostConfig, networkingConfig, name)
			if retryErr != nil {
				return nil, retryErr
			}
		} else {
			return nil, err
		}
	}

	for _, warning := range response.Warnings {
		fmt.Fprintf(stderr, "WARNING: %s\n", warning)
	}
	if containerIDFile != nil {
		if err = containerIDFile.Write(response.ID); err != nil {
			return nil, err
		}
	}
	return &response, nil
}
