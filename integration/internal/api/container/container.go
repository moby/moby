package container

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// CreateVMemLabel creates a container with the supplied parameters
func CreateVMemLabel(client client.APIClient, v []string, memoryMB int, labels map[string]string, img string, cmd []string) (string, error) {
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

// Run runs the command provided in the container image named
func Run(client client.APIClient, img string, cmd []string) (string, error) {
	return RunV(client, []string{}, img, cmd)
}

// RunV runs the command provided in the container image named with
// the equivalent of -v bind mounts
func RunV(client client.APIClient, v []string, img string, cmd []string) (string, error) {
	return RunVMem(client, v, 0, img, cmd)
}

// RunVMem runs the command provided in the container image named with
// the equivalent of -v bind mounts and a specified memory limit
func RunVMem(client client.APIClient, v []string, memoryMB int, img string, cmd []string) (string, error) {
	return RunVMemLabel(client, v, memoryMB, map[string]string{}, img, cmd)
}

// RunVLabel runs the command provided in the container image named
// with the equivalent of -v bind mounts and the specified labels
func RunVLabel(client client.APIClient, v []string, labels map[string]string, img string, cmd []string) (string, error) {
	return RunVMemLabel(client, v, 0, labels, img, cmd)
}

// RunVMemLabel runs the command provided in the container image named
// with the equivalent of -v bind mounts, a specified memory limit,
// and the specified labels
func RunVMemLabel(client client.APIClient, v []string, memoryMB int, labels map[string]string, img string, cmd []string) (string, error) {
	containerID, err := CreateVMemLabel(client, v, memoryMB, labels, img, cmd)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	if err := client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return containerID, nil
}

// Wait waits until the named container has exited
func Wait(client client.APIClient, container string) (int64, error) {
	resultC, errC := client.ContainerWait(context.Background(), container, "")

	select {
	case result := <-resultC:
		return result.StatusCode, nil
	case err := <-errC:
		return -1, err
	}
}

// Stop stops the named container
func Stop(client client.APIClient, container string) error {
	timeout := time.Duration(10) * time.Second
	ctx := context.Background()
	return client.ContainerStop(ctx, container, &timeout)
}

// Start starts the named container
func Start(client client.APIClient, container string) error {
	ctx := context.Background()
	return client.ContainerStart(ctx, container, types.ContainerStartOptions{})
}

// Kill kills the named container with SIGKILL
func Kill(client client.APIClient, container string) error {
	ctx := context.Background()
	return client.ContainerKill(ctx, container, "KILL")
}

// Export exports a container's file system as a tarball
func Export(client client.APIClient, path, name string) error {
	ctx := context.Background()
	responseReader, err := client.ContainerExport(ctx, name)
	if err != nil {
		return err
	}
	defer responseReader.Close()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, responseReader)
	return err
}
