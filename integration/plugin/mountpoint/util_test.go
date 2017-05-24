package mountpoint

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// containerCreateVMemLabel creates a container with the supplied parameters
func containerCreateVMemLabel(client client.APIClient, v []string, memoryMB int, labels map[string]string, img string, cmd []string) (string, error) {
	ctx := context.Background()
	config := container.Config{
		Cmd:    cmd,
		Image:  img,
		Labels: labels,
	}
	hostConfig := container.HostConfig{
		Binds: v,
		Resources: container.Resources{
			Memory: int64(memoryMB * 1024 * 1024),
		},
	}
	networkingConfig := networktypes.NetworkingConfig{}
	name := ""
	response, err := client.ContainerCreate(ctx, &config, &hostConfig, &networkingConfig, name)

	if err != nil {
		return "", err
	}

	return response.ID, nil
}

// containerRun runs the command provided in the container image named
func containerRun(client client.APIClient, img string, cmd []string) (string, error) {
	return containerRunV(client, []string{}, img, cmd)
}

// containerRunV runs the command provided in the container image named with
// the equivalent of -v bind mounts
func containerRunV(client client.APIClient, v []string, img string, cmd []string) (string, error) {
	return containerRunVMem(client, v, 0, img, cmd)
}

// containerRunVMem runs the command provided in the container image named with
// the equivalent of -v bind mounts and a specified memory limit
func containerRunVMem(client client.APIClient, v []string, memoryMB int, img string, cmd []string) (string, error) {
	return containerRunVMemLabel(client, v, memoryMB, map[string]string{}, img, cmd)
}

// containerRunVLabel runs the command provided in the container image named
// with the equivalent of -v bind mounts and the specified labels
func containerRunVLabel(client client.APIClient, v []string, labels map[string]string, img string, cmd []string) (string, error) {
	return containerRunVMemLabel(client, v, 0, labels, img, cmd)
}

// containerRunVMemLabel runs the command provided in the container image named
// with the equivalent of -v bind mounts, a specified memory limit,
// and the specified labels
func containerRunVMemLabel(client client.APIClient, v []string, memoryMB int, labels map[string]string, img string, cmd []string) (string, error) {
	containerID, err := containerCreateVMemLabel(client, v, memoryMB, labels, img, cmd)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	if err := client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return containerID, nil
}

// containerVolumeCreate creates a volume using the named driver with the
// specified options
func containerVolumeCreate(client client.APIClient, driver string, opts map[string]string) (string, error) {
	volReq := volumetypes.VolumesCreateBody{
		Driver:     driver,
		DriverOpts: opts,
		Name:       "",
	}

	vol, err := client.VolumeCreate(context.Background(), volReq)
	if err != nil {
		return "", err
	}
	return vol.Name, nil
}

// containerWait waits until the named container has exited
func containerWait(client client.APIClient, container string) (int64, error) {
	resultC, errC := client.ContainerWait(context.Background(), container, "")

	select {
	case result := <-resultC:
		return result.StatusCode, nil
	case err := <-errC:
		return -1, err
	}
}

// containerStop stops the named container
func containerStop(client client.APIClient, container string) error {
	timeout := time.Duration(10) * time.Second
	ctx := context.Background()
	return client.ContainerStop(ctx, container, &timeout)
}

// containerStart starts the named container
func containerStart(client client.APIClient, container string) error {
	ctx := context.Background()
	return client.ContainerStart(ctx, container, types.ContainerStartOptions{})
}

// containerKill kills the named container with SIGKILL
func containerKill(client client.APIClient, container string) error {
	ctx := context.Background()
	return client.ContainerKill(ctx, container, "KILL")
}
