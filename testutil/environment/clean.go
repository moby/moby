package environment // import "github.com/docker/docker/testutil/environment"

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"gotest.tools/v3/assert"
)

// Clean the environment, preserving protected objects (images, containers, ...)
// and removing everything else. It's meant to run after any tests so that they don't
// depend on each others.
func (e *Execution) Clean(t testing.TB) {
	t.Helper()
	client := e.APIClient()

	platform := e.OSType
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

func unpauseAllContainers(t testing.TB, client client.ContainerAPIClient) {
	t.Helper()
	ctx := context.Background()
	containers := getPausedContainers(ctx, t, client)
	if len(containers) > 0 {
		for _, container := range containers {
			err := client.ContainerUnpause(ctx, container.ID)
			assert.Check(t, err, "failed to unpause container %s", container.ID)
		}
	}
}

func getPausedContainers(ctx context.Context, t testing.TB, client client.ContainerAPIClient) []types.Container {
	t.Helper()
	filter := filters.NewArgs()
	filter.Add("status", "paused")
	containers, err := client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filter,
		All:     true,
	})
	assert.Check(t, err, "failed to list containers")
	return containers
}

var alreadyExists = regexp.MustCompile(`Error response from daemon: removal of container (\w+) is already in progress`)

func deleteAllContainers(t testing.TB, apiclient client.ContainerAPIClient, protectedContainers map[string]struct{}) {
	t.Helper()
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
		if err == nil || client.IsErrNotFound(err) || alreadyExists.MatchString(err.Error()) || isErrNotFoundSwarmClassic(err) {
			continue
		}
		assert.Check(t, err, "failed to remove %s", container.ID)
	}
}

func getAllContainers(ctx context.Context, t testing.TB, client client.ContainerAPIClient) []types.Container {
	t.Helper()
	containers, err := client.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	assert.Check(t, err, "failed to list containers")
	return containers
}

func deleteAllImages(t testing.TB, apiclient client.ImageAPIClient, protectedImages map[string]struct{}) {
	t.Helper()
	images, err := apiclient.ImageList(context.Background(), types.ImageListOptions{})
	assert.Check(t, err, "failed to list images")

	ctx := context.Background()
	for _, image := range images {
		tags := tagsFromImageSummary(image)
		if len(tags) == 0 {
			removeImage(ctx, t, apiclient, image.ID)
			continue
		}
		for _, tag := range tags {
			if _, ok := protectedImages[tag]; !ok {
				removeImage(ctx, t, apiclient, tag)
			}
		}
	}
}

func removeImage(ctx context.Context, t testing.TB, apiclient client.ImageAPIClient, ref string) {
	t.Helper()
	_, err := apiclient.ImageRemove(ctx, ref, types.ImageRemoveOptions{
		Force: true,
	})
	if client.IsErrNotFound(err) {
		return
	}
	assert.Check(t, err, "failed to remove image %s", ref)
}

func deleteAllVolumes(t testing.TB, c client.VolumeAPIClient, protectedVolumes map[string]struct{}) {
	t.Helper()
	volumes, err := c.VolumeList(context.Background(), filters.Args{})
	assert.Check(t, err, "failed to list volumes")

	for _, v := range volumes.Volumes {
		if _, ok := protectedVolumes[v.Name]; ok {
			continue
		}
		err := c.VolumeRemove(context.Background(), v.Name, true)
		// Docker EE may list volumes that no longer exist.
		if isErrNotFoundSwarmClassic(err) {
			continue
		}
		assert.Check(t, err, "failed to remove volume %s", v.Name)
	}
}

func deleteAllNetworks(t testing.TB, c client.NetworkAPIClient, daemonPlatform string, protectedNetworks map[string]struct{}) {
	t.Helper()
	networks, err := c.NetworkList(context.Background(), types.NetworkListOptions{})
	assert.Check(t, err, "failed to list networks")

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
		assert.Check(t, err, "failed to remove network %s", n.ID)
	}
}

func deleteAllPlugins(t testing.TB, c client.PluginAPIClient, protectedPlugins map[string]struct{}) {
	t.Helper()
	plugins, err := c.PluginList(context.Background(), filters.Args{})
	// Docker EE does not allow cluster-wide plugin management.
	if client.IsErrNotImplemented(err) {
		return
	}
	assert.Check(t, err, "failed to list plugins")

	for _, p := range plugins {
		if _, ok := protectedPlugins[p.Name]; ok {
			continue
		}
		err := c.PluginRemove(context.Background(), p.Name, types.PluginRemoveOptions{Force: true})
		assert.Check(t, err, "failed to remove plugin %s", p.ID)
	}
}

// Swarm classic aggregates node errors and returns a 500 so we need to check
// the error string instead of just IsErrNotFound().
func isErrNotFoundSwarmClassic(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such")
}
