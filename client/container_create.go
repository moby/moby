package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
)

type configWrapper struct {
	*container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

// ContainerCreate creates a new container based in the given configuration.
// It can be associated with a name, but it's not mandatory.
func (cli *Client) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error) {
	var response container.ContainerCreateCreatedBody

	if err := cli.NewVersionError("1.25", "stop timeout"); config != nil && config.StopTimeout != nil && err != nil {
		return response, err
	}

	// When using API 1.24 and under, the client is responsible for removing the container
	if hostConfig != nil && versions.LessThan(cli.ClientVersion(), "1.25") {
		hostConfig.AutoRemove = false
	}

	query := url.Values{}
	if containerName != "" {
		query.Set("name", containerName)
	}

	if config != nil {
		resolvedImage, err := cli.getImage(ctx, config)
		if err != nil {
			return response, err
		}
		config.Image = resolvedImage
	}

	body := configWrapper{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}

	serverResp, err := cli.post(ctx, "/containers/create", query, body, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&response)
	return response, err
}

func (cli *Client) getImage(ctx context.Context, config *container.Config) (string, error) {
	allImages, err := cli.ImageList(ctx, types.ImageListOptions{
		All: true,
	})

	if err != nil {
		return "", err
	}
	// Split the image name and tag. For example if the image provided is  d2a6510f3d9c:latest1, image name will  be d2a6510f3d9c and tag will be latest1.
	// By default, tag is "latest".
	imageData := strings.Split(config.Image, ":")
	resolvedImageName, resolvedImageTag := imageData[0], "latest"
	if len(imageData) > 1 {
		resolvedImageTag = imageData[1]
	}
	for _, image := range allImages {
		// We check here if the image is SHA256 Image ID format.
		if strings.HasPrefix(strings.Split(image.ID, ":")[1], resolvedImageName) {
			// If SHA format, then check for the tag if present. By default, "latest" tag is considered,
			for _, repoTag := range image.RepoTags {
				if strings.Split(repoTag, ":")[1] == resolvedImageTag {
					return repoTag, nil
				}
			}
		}
		if len(image.RepoTags) > 0 {
			for _, tag := range image.RepoTags {
				if tag == config.Image {
					return config.Image, nil
				}
			}
		}
	}
	return config.Image, nil
}
