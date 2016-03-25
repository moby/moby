package client

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	networktypes "github.com/docker/engine-api/types/network"
)

func (cli *DockerCli) pullImage(image string, out io.Writer) error {
	ref, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	var tag string
	switch x := reference.WithDefaultTag(ref).(type) {
	case reference.Canonical:
		tag = x.Digest().String()
	case reference.NamedTagged:
		tag = x.Tag()
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return err
	}

	authConfig := cli.resolveAuthConfig(repoInfo.Index)
	encodedAuth, err := encodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImageCreateOptions{
		Parent:       ref.Name(),
		Tag:          tag,
		RegistryAuth: encodedAuth,
	}

	responseBody, err := cli.client.ImageCreate(context.Background(), options)
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

func (cli *DockerCli) createContainer(config *container.Config, hostConfig *container.HostConfig, networkingConfig *networktypes.NetworkingConfig, cidfile, name string) (*types.ContainerCreateResponse, error) {
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
			trustedRef, err = cli.trustedReference(ref)
			if err != nil {
				return nil, err
			}
			config.Image = trustedRef.String()
		}
	}

	//create the container
	response, err := cli.client.ContainerCreate(context.Background(), config, hostConfig, networkingConfig, name)

	//if image not found try to pull it
	if err != nil {
		if client.IsErrImageNotFound(err) && ref != nil {
			fmt.Fprintf(cli.err, "Unable to find image '%s' locally\n", ref.String())

			// we don't want to write to stdout anything apart from container.ID
			if err = cli.pullImage(config.Image, cli.err); err != nil {
				return nil, err
			}
			if ref, ok := ref.(reference.NamedTagged); ok && trustedRef != nil {
				if err := cli.tagTrusted(trustedRef, ref); err != nil {
					return nil, err
				}
			}
			// Retry
			var retryErr error
			response, retryErr = cli.client.ContainerCreate(context.Background(), config, hostConfig, networkingConfig, name)
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

	// These are flags not stored in Config/HostConfig
	var (
		flName = cmd.String([]string{"-name"}, "", "Assign a name to the container")
	)

	config, hostConfig, networkingConfig, cmd, err := runconfigopts.Parse(cmd, args)

	if err != nil {
		cmd.ReportError(err.Error(), true)
		os.Exit(1)
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}
	response, err := cli.createContainer(config, hostConfig, networkingConfig, hostConfig.ContainerIDFile, *flName)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", response.ID)
	return nil
}
