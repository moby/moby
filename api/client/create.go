package client

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/jsonmessage"
	// FIXME migrate to docker/distribution/reference
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	//runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
)

func (cli *DockerCli) pullImage(ctx context.Context, image string, out io.Writer) error {
	ref, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}

	authConfig := cli.ResolveAuthConfig(ctx, repoInfo.Index)
	encodedAuth, err := EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImageCreateOptions{
		RegistryAuth: encodedAuth,
	}

	responseBody, err := cli.client.ImageCreate(ctx, image, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, out, cli.outFd, cli.isTerminalOut, nil)
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

// CreateContainer creates a container from a config
// TODO: this can be unexported again once all container commands are under
// api/client/container
func (cli *DockerCli) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *networktypes.NetworkingConfig, cidfile, name string) (*types.ContainerCreateResponse, error) {
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

		if ref, ok := ref.(reference.NamedTagged); ok && isTrusted() {
			var err error
			trustedRef, err = cli.trustedReference(ctx, ref)
			if err != nil {
				return nil, err
			}
			config.Image = trustedRef.String()
		}
	}

	//create the container
	response, err := cli.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, name)

	//if image not found try to pull it
	if err != nil {
		if client.IsErrImageNotFound(err) && ref != nil {
			fmt.Fprintf(cli.err, "Unable to find image '%s' locally\n", ref.String())

			// we don't want to write to stdout anything apart from container.ID
			if err = cli.pullImage(ctx, config.Image, cli.err); err != nil {
				return nil, err
			}
			if ref, ok := ref.(reference.NamedTagged); ok && trustedRef != nil {
				if err := cli.tagTrusted(ctx, trustedRef, ref); err != nil {
					return nil, err
				}
			}
			// Retry
			var retryErr error
			response, retryErr = cli.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, name)
			if retryErr != nil {
				return nil, retryErr
			}
		} else {
			return nil, err
		}
	}

	for _, warning := range response.Warnings {
		fmt.Fprintf(cli.err, "WARNING: %s\n", warning)
	}
	if containerIDFile != nil {
		if err = containerIDFile.Write(response.ID); err != nil {
			return nil, err
		}
	}
	return &response, nil
}

// CmdCreate creates a new container from a given image.
//
// Usage: docker create [OPTIONS] IMAGE [COMMAND] [ARG...]
func (cli *DockerCli) CmdCreate(args ...string) error {
	cmd := Cli.Subcmd("create", []string{"IMAGE [COMMAND] [ARG...]"}, Cli.DockerCommands["create"].Description, true)
	addTrustedFlags(cmd, true)

	// TODO: tmp disable for PoC, convert to cobra and pflag later
	// These are flags not stored in Config/HostConfig
	//	var (
	//		flName = cmd.String([]string{"-name"}, "", "Assign a name to the container")
	//	)

	//	config, hostConfig, networkingConfig, cmd, err := runconfigopts.Parse(cmd, args)
	//
	//	if err != nil {
	//		cmd.ReportError(err.Error(), true)
	//		os.Exit(1)
	//	}
	//	if config.Image == "" {
	//		cmd.Usage()
	//		return nil
	//	}
	//	response, err := cli.CreateContainer(context.Background(), config, hostConfig, networkingConfig, hostConfig.ContainerIDFile, *flName)
	//	if err != nil {
	//		return err
	//	}
	//	fmt.Fprintf(cli.out, "%s\n", response.ID)
	return nil
}
