package environment

import (
	"context"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
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

func unpauseAllContainers(ctx context.Context, t testing.TB, apiClient client.ContainerAPIClient) {
	t.Helper()
	containers := getPausedContainers(ctx, t, apiClient)
	if len(containers) > 0 {
		for _, ctr := range containers {
			_, err := apiClient.ContainerUnpause(ctx, ctr.ID, client.ContainerUnpauseOptions{})
			assert.Check(t, err, "failed to unpause container %s", ctr.ID)
		}
	}
}

func getPausedContainers(ctx context.Context, t testing.TB, apiClient client.ContainerAPIClient) []container.Summary {
	t.Helper()
	list, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		Filters: make(client.Filters).Add("status", "paused"),
		All:     true,
	})
	assert.Check(t, err, "failed to list containers")
	return list.Items
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
		_, err := apiclient.ContainerRemove(ctx, ctr.ID, client.ContainerRemoveOptions{
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

func getAllContainers(ctx context.Context, t testing.TB, apiClient client.ContainerAPIClient) []container.Summary {
	t.Helper()
	list, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All: true,
	})
	assert.Check(t, err, "failed to list containers")
	return list.Items
}

func deleteAllImages(ctx context.Context, t testing.TB, apiclient client.ImageAPIClient, protectedImages map[string]struct{}) {
	t.Helper()
	imageList, err := apiclient.ImageList(ctx, client.ImageListOptions{})
	assert.Check(t, err, "failed to list images")

	for _, img := range imageList.Items {
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
	res, err := c.VolumeList(ctx, client.VolumeListOptions{})
	assert.Check(t, err, "failed to list volumes")

	for _, v := range res.Items {
		if _, ok := protectedVolumes[v.Name]; ok {
			continue
		}
		_, err := c.VolumeRemove(ctx, v.Name, client.VolumeRemoveOptions{
			Force: true,
		})
		assert.Check(t, err, "failed to remove volume %s", v.Name)
	}
}

func deleteAllNetworks(ctx context.Context, t testing.TB, c client.NetworkAPIClient, daemonPlatform string, protectedNetworks map[string]struct{}) {
	t.Helper()
	res, err := c.NetworkList(ctx, client.NetworkListOptions{})
	assert.Check(t, err, "failed to list networks")

	for _, nw := range res.Items {
		if nw.Name == network.NetworkBridge || nw.Name == network.NetworkNone || nw.Name == network.NetworkHost {
			continue
		}
		if _, ok := protectedNetworks[nw.ID]; ok {
			continue
		}
		if daemonPlatform == "windows" && strings.ToLower(nw.Name) == network.NetworkNat {
			// nat is a pre-defined network on Windows and cannot be removed
			continue
		}
		_, err := c.NetworkRemove(ctx, nw.ID, client.NetworkRemoveOptions{})
		assert.Check(t, err, "failed to remove network %s", nw.ID)
	}
}

func deleteAllPlugins(ctx context.Context, t testing.TB, c client.PluginAPIClient, protectedPlugins map[string]struct{}) {
	t.Helper()
	res, err := c.PluginList(ctx, client.PluginListOptions{})
	// Docker EE does not allow cluster-wide plugin management.
	if cerrdefs.IsNotImplemented(err) {
		return
	}
	assert.Check(t, err, "failed to list plugins")

	for _, p := range res.Items {
		if _, ok := protectedPlugins[p.Name]; ok {
			continue
		}
		_, err := c.PluginRemove(ctx, p.Name, client.PluginRemoveOptions{Force: true})
		assert.Check(t, err, "failed to remove plugin %s", p.ID)
	}
}
