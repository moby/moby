package environment

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

// ProtectImage adds the specified image(s) to be protected in case of clean
func (e *Execution) ProtectImage(t testingT, images ...string) {
	for _, image := range images {
		imageInspect, _, err := e.client.ImageInspectWithRaw(context.Background(), image)
		if err != nil {
			t.Fatalf("error inspecting image %s: %v", image, err)
		}
		e.protectedElements.images[imageInspect.ID] = struct{}{}
	}
}

// ProtectedElements holds images, containers, volumes, networks and plugins to protect when cleaning the environment
type ProtectedElements struct {
	containers map[string]struct{}
	images     map[string]struct{}
	volumes    map[string]struct{}
	networks   map[string]struct{}
	plugins    map[string]struct{}
}

func listExistingElement(cli client.APIClient) (*ProtectedElements, error) {
	protectedElts := &ProtectedElements{
		containers: map[string]struct{}{},
		images:     map[string]struct{}{},
		volumes:    map[string]struct{}{},
		networks:   map[string]struct{}{},
		plugins:    map[string]struct{}{},
	}

	protects := []struct {
		fn    func(ctx context.Context, apiClient client.APIClient) ([]string, error)
		field map[string]struct{}
	}{
		{listImages, protectedElts.images},
		{listContainers, protectedElts.containers},
		{listVolumes, protectedElts.volumes},
		{listNetworks, protectedElts.networks},
		{listPlugins, protectedElts.plugins},
	}

	ctx := context.Background()

	for _, protect := range protects {
		m, err := protect.fn(ctx, cli)
		if err != nil {
			return nil, err
		}
		for _, value := range m {
			protect.field[value] = struct{}{}
		}
	}
	return protectedElts, nil
}

func listImages(ctx context.Context, cli client.APIClient) ([]string, error) {
	images := []string{}
	imageList, err := cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return images, err
	}
	for _, image := range imageList {
		images = append(images, image.ID)
	}
	return images, nil
}

func listContainers(ctx context.Context, cli client.APIClient) ([]string, error) {
	containers := []string{}
	containerList, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return containers, err
	}
	for _, container := range containerList {
		containers = append(containers, container.ID)
	}
	return containers, nil
}

func listPausedContainers(ctx context.Context, cli client.APIClient) ([]string, error) {
	containers := []string{}
	filter := filters.NewArgs()
	filter.Add("status", "paused")
	containerList, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return containers, err
	}
	for _, container := range containerList {
		containers = append(containers, container.ID)
	}
	return containers, nil
}

func listVolumes(ctx context.Context, cli client.APIClient) ([]string, error) {
	volumes := []string{}
	volumeList, err := cli.VolumeList(ctx, filters.NewArgs())
	if err != nil {
		return volumes, err
	}
	for _, volume := range volumeList.Volumes {
		volumes = append(volumes, volume.Name)
	}
	return volumes, nil
}

func listNetworks(ctx context.Context, cli client.APIClient) ([]string, error) {
	networks := []string{}
	networkList, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return networks, err
	}
	for _, network := range networkList {
		networks = append(networks, network.ID)
	}
	return networks, nil
}

func listPlugins(ctx context.Context, cli client.APIClient) ([]string, error) {
	plugins := []string{}
	pluginList, err := cli.PluginList(ctx, filters.NewArgs())
	if err != nil {
		return plugins, err
	}
	for _, plugin := range pluginList {
		plugins = append(plugins, plugin.ID)
	}
	return plugins, nil
}
