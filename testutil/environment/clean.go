package environment

import (
	"context"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"go.opentelemetry.io/otel"
	"gotest.tools/v3/assert"
)

// Clean the environment, preserving protected objects (images, containers, ...)
// and removing everything else. It's meant to run after any tests so that they don't
// depend on each others.
func (e *Execution) Clean(ctx context.Context, t testing.TB) {
	t.Helper()

	ctx, span := otel.Tracer("").Start(ctx, "CleanupEnvironment")
	defer span.End()

	apiClient := e.APIClient()

	platform := e.DaemonInfo.OSType
	if platform != "windows" || e.DaemonInfo.Isolation == "hyperv" {
		unpauseAllContainers(ctx, t, apiClient)
	}
	deleteAllContainers(ctx, t, apiClient, e.protectedElements.containers)
	deleteAllImages(ctx, t, apiClient, e.protectedElements.images)
	deleteAllVolumes(ctx, t, apiClient, e.protectedElements.volumes)
	deleteAllNetworks(ctx, t, apiClient, platform, e.protectedElements.networks)
	if platform == "linux" {
		deleteAllPlugins(ctx, t, apiClient, e.protectedElements.plugins)
		restoreDefaultBridge(t, e.protectedElements.defaultBridgeInfo)
	}
}

func unpauseAllContainers(ctx context.Context, t testing.TB, client client.ContainerAPIClient) {
	t.Helper()
	containers := getPausedContainers(ctx, t, client)
	if len(containers) > 0 {
		for _, ctr := range containers {
			err := client.ContainerUnpause(ctx, ctr.ID)
			assert.Check(t, err, "failed to unpause container %s", ctr.ID)
		}
	}
}

func getPausedContainers(ctx context.Context, t testing.TB, client client.ContainerAPIClient) []container.Summary {
	t.Helper()
	containers, err := client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "paused")),
		All:     true,
	})
	assert.Check(t, err, "failed to list containers")
	return containers
}

func deleteAllContainers(ctx context.Context, t testing.TB, apiclient client.ContainerAPIClient, protectedContainers map[string]struct{}) {
	t.Helper()
	containers := getAllContainers(ctx, t, apiclient)
	if len(containers) == 0 {
		return
	}

	for _, ctr := range containers {
		if _, ok := protectedContainers[ctr.ID]; ok {
			continue
		}
		err := apiclient.ContainerRemove(ctx, ctr.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})

		// Ignore if container is already gone, or removal of container is already in progress.
		if err == nil || cerrdefs.IsNotFound(err) || strings.Contains(err.Error(), "is already in progress") {
			continue
		}
		assert.Check(t, err, "failed to remove %s", ctr.ID)
	}
}

func getAllContainers(ctx context.Context, t testing.TB, client client.ContainerAPIClient) []container.Summary {
	t.Helper()
	containers, err := client.ContainerList(ctx, container.ListOptions{
		All: true,
	})
	assert.Check(t, err, "failed to list containers")
	return containers
}

func deleteAllImages(ctx context.Context, t testing.TB, apiclient client.ImageAPIClient, protectedImages map[string]struct{}) {
	t.Helper()
	images, err := apiclient.ImageList(ctx, client.ImageListOptions{})
	assert.Check(t, err, "failed to list images")

	for _, img := range images {
		tags := tagsFromImageSummary(img)
		if _, ok := protectedImages[img.ID]; ok {
			continue
		}
		if len(tags) == 0 {
			removeImage(ctx, t, apiclient, img.ID)
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
	_, err := apiclient.ImageRemove(ctx, ref, client.ImageRemoveOptions{
		Force: true,
	})
	if cerrdefs.IsNotFound(err) {
		return
	}
	assert.Check(t, err, "failed to remove image %s", ref)
}

func deleteAllVolumes(ctx context.Context, t testing.TB, c client.VolumeAPIClient, protectedVolumes map[string]struct{}) {
	t.Helper()
	volumes, err := c.VolumeList(ctx, client.VolumeListOptions{})
	assert.Check(t, err, "failed to list volumes")

	for _, v := range volumes.Volumes {
		if _, ok := protectedVolumes[v.Name]; ok {
			continue
		}
		err := c.VolumeRemove(ctx, v.Name, true)
		assert.Check(t, err, "failed to remove volume %s", v.Name)
	}
}

func deleteAllNetworks(ctx context.Context, t testing.TB, c client.NetworkAPIClient, daemonPlatform string, protectedNetworks map[string]struct{}) {
	t.Helper()
	networks, err := c.NetworkList(ctx, client.NetworkListOptions{})
	assert.Check(t, err, "failed to list networks")

	for _, n := range networks {
		if n.Name == network.NetworkBridge || n.Name == network.NetworkNone || n.Name == network.NetworkHost {
			continue
		}
		if _, ok := protectedNetworks[n.ID]; ok {
			continue
		}
		if daemonPlatform == "windows" && strings.ToLower(n.Name) == network.NetworkNat {
			// nat is a pre-defined network on Windows and cannot be removed
			continue
		}
		err := c.NetworkRemove(ctx, n.ID)
		assert.Check(t, err, "failed to remove network %s", n.ID)
	}
}

func deleteAllPlugins(ctx context.Context, t testing.TB, c client.PluginAPIClient, protectedPlugins map[string]struct{}) {
	t.Helper()
	plugins, err := c.PluginList(ctx, filters.Args{})
	// Docker EE does not allow cluster-wide plugin management.
	if cerrdefs.IsNotImplemented(err) {
		return
	}
	assert.Check(t, err, "failed to list plugins")

	for _, p := range plugins {
		if _, ok := protectedPlugins[p.Name]; ok {
			continue
		}
		err := c.PluginRemove(ctx, p.Name, client.PluginRemoveOptions{Force: true})
		assert.Check(t, err, "failed to remove plugin %s", p.ID)
	}
}
