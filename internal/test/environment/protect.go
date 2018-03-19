package environment // import "github.com/docker/docker/internal/test/environment"

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dclient "github.com/docker/docker/client"
	"github.com/gotestyourself/gotestyourself/assert"
)

var frozenImages = []string{"busybox:latest", "busybox:glibc", "hello-world:frozen", "debian:jessie"}

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
func ProtectAll(t testingT, testEnv *Execution) {
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
func (e *Execution) ProtectContainer(t testingT, containers ...string) {
	for _, container := range containers {
		e.protectedElements.containers[container] = struct{}{}
	}
}

// ProtectContainers protects existing containers from being cleaned up at the
// end of test runs
func ProtectContainers(t testingT, testEnv *Execution) {
	containers := getExistingContainers(t, testEnv)
	testEnv.ProtectContainer(t, containers...)
}

func getExistingContainers(t assert.TestingT, testEnv *Execution) []string {
	client := testEnv.APIClient()
	containerList, err := client.ContainerList(context.Background(), types.ContainerListOptions{
		All: true,
	})
	assert.NilError(t, err, "failed to list containers")

	containers := []string{}
	for _, container := range containerList {
		containers = append(containers, container.ID)
	}
	return containers
}

// ProtectImage adds the specified image(s) to be protected in case of clean
func (e *Execution) ProtectImage(t testingT, images ...string) {
	for _, image := range images {
		e.protectedElements.images[image] = struct{}{}
	}
}

// ProtectImages protects existing images and on linux frozen images from being
// cleaned up at the end of test runs
func ProtectImages(t testingT, testEnv *Execution) {
	images := getExistingImages(t, testEnv)

	if testEnv.OSType == "linux" {
		images = append(images, frozenImages...)
	}
	testEnv.ProtectImage(t, images...)
}

func getExistingImages(t assert.TestingT, testEnv *Execution) []string {
	client := testEnv.APIClient()
	filter := filters.NewArgs()
	filter.Add("dangling", "false")
	imageList, err := client.ImageList(context.Background(), types.ImageListOptions{
		All:     true,
		Filters: filter,
	})
	assert.NilError(t, err, "failed to list images")

	images := []string{}
	for _, image := range imageList {
		images = append(images, tagsFromImageSummary(image)...)
	}
	return images
}

func tagsFromImageSummary(image types.ImageSummary) []string {
	result := []string{}
	for _, tag := range image.RepoTags {
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
func (e *Execution) ProtectNetwork(t testingT, networks ...string) {
	for _, network := range networks {
		e.protectedElements.networks[network] = struct{}{}
	}
}

// ProtectNetworks protects existing networks from being cleaned up at the end
// of test runs
func ProtectNetworks(t testingT, testEnv *Execution) {
	networks := getExistingNetworks(t, testEnv)
	testEnv.ProtectNetwork(t, networks...)
}

func getExistingNetworks(t assert.TestingT, testEnv *Execution) []string {
	client := testEnv.APIClient()
	networkList, err := client.NetworkList(context.Background(), types.NetworkListOptions{})
	assert.NilError(t, err, "failed to list networks")

	networks := []string{}
	for _, network := range networkList {
		networks = append(networks, network.ID)
	}
	return networks
}

// ProtectPlugin adds the specified plugin(s) to be protected in case of clean
func (e *Execution) ProtectPlugin(t testingT, plugins ...string) {
	for _, plugin := range plugins {
		e.protectedElements.plugins[plugin] = struct{}{}
	}
}

// ProtectPlugins protects existing plugins from being cleaned up at the end of
// test runs
func ProtectPlugins(t testingT, testEnv *Execution) {
	plugins := getExistingPlugins(t, testEnv)
	testEnv.ProtectPlugin(t, plugins...)
}

func getExistingPlugins(t assert.TestingT, testEnv *Execution) []string {
	client := testEnv.APIClient()
	pluginList, err := client.PluginList(context.Background(), filters.Args{})
	// Docker EE does not allow cluster-wide plugin management.
	if dclient.IsErrNotImplemented(err) {
		return []string{}
	}
	assert.NilError(t, err, "failed to list plugins")

	plugins := []string{}
	for _, plugin := range pluginList {
		plugins = append(plugins, plugin.Name)
	}
	return plugins
}

// ProtectVolume adds the specified volume(s) to be protected in case of clean
func (e *Execution) ProtectVolume(t testingT, volumes ...string) {
	for _, volume := range volumes {
		e.protectedElements.volumes[volume] = struct{}{}
	}
}

// ProtectVolumes protects existing volumes from being cleaned up at the end of
// test runs
func ProtectVolumes(t testingT, testEnv *Execution) {
	volumes := getExistingVolumes(t, testEnv)
	testEnv.ProtectVolume(t, volumes...)
}

func getExistingVolumes(t assert.TestingT, testEnv *Execution) []string {
	client := testEnv.APIClient()
	volumeList, err := client.VolumeList(context.Background(), filters.Args{})
	assert.NilError(t, err, "failed to list volumes")

	volumes := []string{}
	for _, volume := range volumeList.Volumes {
		volumes = append(volumes, volume.Name)
	}
	return volumes
}
