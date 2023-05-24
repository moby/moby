package environment

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

var frozenImages = []string{"busybox:latest", "busybox:glibc", "hello-world:frozen", "debian:bullseye-slim"}

type protectedElements struct {
	containers map[string]struct{}
	images     map[string]struct{}
	networks   map[string]struct{}
	plugins    map[string]struct{}
	volumes    map[string]struct{}
}

func newProtectedElements() protectedElements {
	return protectedElements{
		containers: map[string]struct{}{},
		images:     map[string]struct{}{},
		networks:   map[string]struct{}{},
		plugins:    map[string]struct{}{},
		volumes:    map[string]struct{}{},
	}
}

// ProtectAll protects the existing environment (containers, images, networks,
// volumes, and, on Linux, plugins) from being cleaned up at the end of test
// runs
func ProtectAll(t testing.TB, testEnv *Execution) {
	t.Helper()
	ProtectContainers(t, testEnv)
	ProtectImages(t, testEnv)
	ProtectNetworks(t, testEnv)
	ProtectVolumes(t, testEnv)
	if testEnv.OSType == "linux" {
		ProtectPlugins(t, testEnv)
	}
}

// ProtectContainer adds the specified container(s) to be protected in case of
// clean
func (e *Execution) ProtectContainer(t testing.TB, containers ...string) {
	t.Helper()
	for _, container := range containers {
		e.protectedElements.containers[container] = struct{}{}
	}
}

// ProtectContainers protects existing containers from being cleaned up at the
// end of test runs
func ProtectContainers(t testing.TB, testEnv *Execution) {
	t.Helper()
	containers := getExistingContainers(t, testEnv)
	testEnv.ProtectContainer(t, containers...)
}

func getExistingContainers(t testing.TB, testEnv *Execution) []string {
	t.Helper()
	client := testEnv.APIClient()
	containerList, err := client.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
	})
	assert.NilError(t, err, "failed to list containers")

	var containers []string
	for _, container := range containerList {
		containers = append(containers, container.ID)
	}
	return containers
}

// ProtectImage adds the specified image(s) to be protected in case of clean
func (e *Execution) ProtectImage(t testing.TB, images ...string) {
	t.Helper()
	for _, image := range images {
		e.protectedElements.images[image] = struct{}{}
	}
}

// ProtectImages protects existing images and on linux frozen images from being
// cleaned up at the end of test runs
func ProtectImages(t testing.TB, testEnv *Execution) {
	t.Helper()
	images := getExistingImages(t, testEnv)

	if testEnv.OSType == "linux" {
		images = append(images, frozenImages...)
	}
	testEnv.ProtectImage(t, images...)
	testEnv.ProtectImage(t, DanglingImageIdGraphDriver, DanglingImageIdSnapshotter)
}

func getExistingImages(t testing.TB, testEnv *Execution) []string {
	t.Helper()
	client := testEnv.APIClient()
	imageList, err := client.ImageList(context.Background(), types.ImageListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("dangling", "false")),
	})
	assert.NilError(t, err, "failed to list images")

	var images []string
	for _, image := range imageList {
		images = append(images, tagsFromImageSummary(image)...)
	}
	return images
}

func tagsFromImageSummary(image types.ImageSummary) []string {
	var result []string
	for _, tag := range image.RepoTags {
		// Starting from API 1.43 no longer outputs the hardcoded <none>
		// strings. But since the tests might be ran against a remote
		// daemon/pre 1.43 CLI we must still be able to handle it.
		if tag != "<none>:<none>" {
			result = append(result, tag)
		}
	}
	for _, digest := range image.RepoDigests {
		if digest != "<none>@<none>" {
			result = append(result, digest)
		}
	}
	return result
}

// ProtectNetwork adds the specified network(s) to be protected in case of
// clean
func (e *Execution) ProtectNetwork(t testing.TB, networks ...string) {
	t.Helper()
	for _, network := range networks {
		e.protectedElements.networks[network] = struct{}{}
	}
}

// ProtectNetworks protects existing networks from being cleaned up at the end
// of test runs
func ProtectNetworks(t testing.TB, testEnv *Execution) {
	t.Helper()
	networks := getExistingNetworks(t, testEnv)
	testEnv.ProtectNetwork(t, networks...)
}

func getExistingNetworks(t testing.TB, testEnv *Execution) []string {
	t.Helper()
	client := testEnv.APIClient()
	networkList, err := client.NetworkList(context.Background(), types.NetworkListOptions{})
	assert.NilError(t, err, "failed to list networks")

	var networks []string
	for _, network := range networkList {
		networks = append(networks, network.ID)
	}
	return networks
}

// ProtectPlugin adds the specified plugin(s) to be protected in case of clean
func (e *Execution) ProtectPlugin(t testing.TB, plugins ...string) {
	t.Helper()
	for _, plugin := range plugins {
		e.protectedElements.plugins[plugin] = struct{}{}
	}
}

// ProtectPlugins protects existing plugins from being cleaned up at the end of
// test runs
func ProtectPlugins(t testing.TB, testEnv *Execution) {
	t.Helper()
	plugins := getExistingPlugins(t, testEnv)
	testEnv.ProtectPlugin(t, plugins...)
}

func getExistingPlugins(t testing.TB, testEnv *Execution) []string {
	t.Helper()
	client := testEnv.APIClient()
	pluginList, err := client.PluginList(context.Background(), filters.Args{})
	// Docker EE does not allow cluster-wide plugin management.
	if errdefs.IsNotImplemented(err) {
		return []string{}
	}
	assert.NilError(t, err, "failed to list plugins")

	var plugins []string
	for _, plugin := range pluginList {
		plugins = append(plugins, plugin.Name)
	}
	return plugins
}

// ProtectVolume adds the specified volume(s) to be protected in case of clean
func (e *Execution) ProtectVolume(t testing.TB, volumes ...string) {
	t.Helper()
	for _, vol := range volumes {
		e.protectedElements.volumes[vol] = struct{}{}
	}
}

// ProtectVolumes protects existing volumes from being cleaned up at the end of
// test runs
func ProtectVolumes(t testing.TB, testEnv *Execution) {
	t.Helper()
	volumes := getExistingVolumes(t, testEnv)
	testEnv.ProtectVolume(t, volumes...)
}

func getExistingVolumes(t testing.TB, testEnv *Execution) []string {
	t.Helper()
	client := testEnv.APIClient()
	volumeList, err := client.VolumeList(context.Background(), volume.ListOptions{})
	assert.NilError(t, err, "failed to list volumes")

	var volumes []string
	for _, vol := range volumeList.Volumes {
		volumes = append(volumes, vol.Name)
	}
	return volumes
}
