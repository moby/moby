package environment

import (
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

type testingT interface {
	require.TestingT
	logT
	Fatalf(string, ...interface{})
}

type logT interface {
	Logf(string, ...interface{})
}

// Clean the environment, preserving protected objects (images, containers, ...)
// and removing everything else. It's meant to run after any tests so that they don't
// depend on each others.
func (e *Execution) Clean(t testingT) {
	client := e.APIClient()

	platform := e.DaemonInfo.OSType
	if (platform != "windows") || (platform == "windows" && e.DaemonInfo.Isolation == "hyperv") {
		unpauseAllContainers(t, client)
	}
	deleteAllContainers(t, client, e.protectedElements.containers)
	deleteAllImages(t, client, e.protectedElements.images)
	deleteAllVolumes(t, client, e.protectedElements.volumes)
	deleteAllNetworks(t, client, platform, e.protectedElements.networks)
	if platform == "linux" {
		deleteAllPlugins(t, client, e.protectedElements.plugins)
	}
}

func unpauseAllContainers(t assert.TestingT, client client.ContainerAPIClient) {
	ctx := context.Background()
	containers := getPausedContainers(ctx, t, client)
	if len(containers) > 0 {
		for _, container := range containers {
			err := client.ContainerUnpause(ctx, container.ID)
			assert.NoError(t, err, "failed to unpause container %s", container.ID)
		}
	}
}

func getPausedContainers(ctx context.Context, t assert.TestingT, client client.ContainerAPIClient) []types.Container {
	filter := filters.NewArgs()
	filter.Add("status", "paused")
	containers, err := client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filter,
		Quiet:   true,
		All:     true,
	})
	assert.NoError(t, err, "failed to list containers")
	return containers
}

var alreadyExists = regexp.MustCompile(`Error response from daemon: removal of container (\w+) is already in progress`)

func deleteAllContainers(t assert.TestingT, apiclient client.ContainerAPIClient, protectedContainers map[string]struct{}) {
	ctx := context.Background()
	containers := getAllContainers(ctx, t, apiclient)
	if len(containers) == 0 {
		return
	}

	for _, container := range containers {
		if _, ok := protectedContainers[container.ID]; ok {
			continue
		}
		err := apiclient.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
		if err == nil || client.IsErrNotFound(err) || alreadyExists.MatchString(err.Error()) {
			continue
		}
		assert.NoError(t, err, "failed to remove %s", container.ID)
	}
}

func getAllContainers(ctx context.Context, t assert.TestingT, client client.ContainerAPIClient) []types.Container {
	containers, err := client.ContainerList(ctx, types.ContainerListOptions{
		Quiet: true,
		All:   true,
	})
	assert.NoError(t, err, "failed to list containers")
	return containers
}

func deleteAllImages(t testingT, apiclient client.ImageAPIClient, protectedImages map[string]struct{}) {
	images, err := apiclient.ImageList(context.Background(), types.ImageListOptions{})
	assert.NoError(t, err, "failed to list images")

	ctx := context.Background()
	for _, image := range images {
		tags := tagsFromImageSummary(image)
		if len(tags) == 0 {
			t.Logf("Removing image %s", image.ID)
			removeImage(ctx, t, apiclient, image.ID)
			continue
		}
		for _, tag := range tags {
			if _, ok := protectedImages[tag]; !ok {
				t.Logf("Removing image %s", tag)
				removeImage(ctx, t, apiclient, tag)
				continue
			}
		}
	}
}

func removeImage(ctx context.Context, t assert.TestingT, apiclient client.ImageAPIClient, ref string) {
	_, err := apiclient.ImageRemove(ctx, ref, types.ImageRemoveOptions{
		Force: true,
	})
	if client.IsErrNotFound(err) {
		return
	}
	assert.NoError(t, err, "failed to remove image %s", ref)
}

func deleteAllVolumes(t assert.TestingT, c client.VolumeAPIClient, protectedVolumes map[string]struct{}) {
	volumes, err := c.VolumeList(context.Background(), filters.Args{})
	assert.NoError(t, err, "failed to list volumes")

	for _, v := range volumes.Volumes {
		if _, ok := protectedVolumes[v.Name]; ok {
			continue
		}
		err := c.VolumeRemove(context.Background(), v.Name, true)
		assert.NoError(t, err, "failed to remove volume %s", v.Name)
	}
}

func deleteAllNetworks(t assert.TestingT, c client.NetworkAPIClient, daemonPlatform string, protectedNetworks map[string]struct{}) {
	networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
	assert.NoError(t, err, "failed to list networks")

	for _, n := range networks {
		if n.Name == "bridge" || n.Name == "none" || n.Name == "host" {
			continue
		}
		if _, ok := protectedNetworks[n.ID]; ok {
			continue
		}
		if daemonPlatform == "windows" && strings.ToLower(n.Name) == "nat" {
			// nat is a pre-defined network on Windows and cannot be removed
			continue
		}
		err := c.NetworkRemove(context.Background(), n.ID)
		assert.NoError(t, err, "failed to remove network %s", n.ID)
	}
}

func deleteAllPlugins(t assert.TestingT, c client.PluginAPIClient, protectedPlugins map[string]struct{}) {
	plugins, err := c.PluginList(context.Background(), filters.Args{})
	assert.NoError(t, err, "failed to list plugins")

	for _, p := range plugins {
		if _, ok := protectedPlugins[p.Name]; ok {
			continue
		}
		err := c.PluginRemove(context.Background(), p.Name, types.PluginRemoveOptions{Force: true})
		assert.NoError(t, err, "failed to remove plugin %s", p.ID)
	}
}
