package container

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	clientapi "github.com/docker/engine-api/client"
)

// getExitCode performs an inspect on the container. It returns
// the running state and the exit code.
func getExitCode(dockerCli *client.DockerCli, ctx context.Context, containerID string) (bool, int, error) {
	c, err := dockerCli.Client().ContainerInspect(ctx, containerID)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if err != clientapi.ErrConnectionFailed {
			return false, -1, err
		}
		return false, -1, nil
	}
	return c.State.Running, c.State.ExitCode, nil
}
