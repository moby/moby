package client

import (
	"fmt"
	"io"
	"os"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/image"
	networktypes "github.com/docker/engine-api/types/network"
)

type createConfig struct {
	config           *container.Config              // Config of the contaiiner
	hostConfig       *container.HostConfig          // HostConfig of the container
	networkingConfig *networktypes.NetworkingConfig // NetworkingConfig of the container
	cidfile          string                         // File the where the ContainerID is written
	name             string                         // The name assign to the container
	pull             image.PullBehavior             // How to deal with image pulls
	translator       reference.TranslatorFunc       // Callback for trusted reference conversion
}

func (cli *DockerCli) pullImage(image string) error {
	return cli.pullImageCustomOut(image, cli.out)
}

func (cli *DockerCli) pullImageCustomOut(image string, out io.Writer) error {
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

	// Resolve the Auth config relevant for this server
	encodedAuth, err := cli.encodeRegistryAuth(repoInfo.Index)
	if err != nil {
		return err
	}

	options := types.ImageCreateOptions{
		Parent:       ref.Name(),
		Tag:          tag,
		RegistryAuth: encodedAuth,
	}

	responseBody, err := cli.client.ImageCreate(options)
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

func (cli *DockerCli) createContainer(createConfig *createConfig) (*types.ContainerCreateResponse, error) {
	config := createConfig.config
	var containerIDFile *cidFile

	if createConfig.cidfile != "" {
		var err error
		if containerIDFile, err = newCIDFile(createConfig.cidfile); err != nil {
			return nil, err
		}
		defer containerIDFile.Close()
	}

	ref, err := reference.ParseNamed(config.Image)
	if err != nil {
		return nil, err
	}
	ref = reference.WithDefaultTag(ref)

	var (
		namedTaggedRef reference.NamedTagged
		trustedRef     reference.Canonical
	)
	if createConfig.translator != nil {
		// Updating config.Image to the trusted (notary) reference
		// should be sufficient to deal with --pull=true on the first create attempt.
		// Note: This update is only attempted in the case of a NamedTagged reference.
		var ok bool
		if namedTaggedRef, ok = ref.(reference.NamedTagged); ok {
			trustedRef, err = createConfig.translator(namedTaggedRef)
			if err != nil {
				return nil, err
			}
			config.Image = trustedRef.String()
		}
	}
	if createConfig.pull == image.PullAlways {
		if err := cli.pullImageCustomOut(config.Image, cli.err); err != nil {
			return nil, err
		}
	}

	//create the container
	response, err := cli.client.ContainerCreate(config, createConfig.hostConfig, createConfig.networkingConfig, createConfig.name)

	if err != nil {
		// deal with image not found case, return error for anything else.
		if !client.IsErrImageNotFound(err) {
			return nil, err
		}

		fmt.Fprintf(cli.err, "Unable to find image '%s' locally\n", ref.String())

		if createConfig.pull != image.PullMissing {
			return nil, err
		}

		// we don't want to write to stdout anything apart from container.ID
		if err = cli.pullImageCustomOut(config.Image, cli.err); err != nil {
			return nil, err
		}
		if trustedRef != nil {
			// We successfully pulled an updated trusted image for the given named reference.
			// Tag it with the trusted canonical reference.
			if err := cli.tagTrusted(trustedRef, namedTaggedRef); err != nil {
				return nil, err
			}
		}
		// Retry
		var retryErr error
		response, retryErr = cli.client.ContainerCreate(config, createConfig.hostConfig, createConfig.networkingConfig, createConfig.name)
		if retryErr != nil {
			return nil, retryErr
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
	flPull := addPullFlag(cmd)
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

	pullBehavior, translator := cli.trustedPullBehavior(flPull.Val())
	createConfig := &createConfig{
		config:           config,
		hostConfig:       hostConfig,
		networkingConfig: networkingConfig,
		cidfile:          hostConfig.ContainerIDFile,
		name:             *flName,
		pull:             pullBehavior,
		translator:       translator,
	}
	response, err := cli.createContainer(createConfig)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", response.ID)
	return nil
}
