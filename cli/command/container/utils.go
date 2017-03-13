package container

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/cli/command"
	clientapi "github.com/docker/docker/client"
	"golang.org/x/net/context"
)

func waitExitOrRemoved(ctx context.Context, dockerCli *command.DockerCli, containerID string, waitRemove bool) chan int {
	if len(containerID) == 0 {
		// containerID can never be empty
		panic("Internal Error: waitExitOrRemoved needs a containerID as parameter")
	}

	waitChan := make(chan int)
	waitCtx, cancel := context.WithCancel(ctx)

	// Spawn a goroutine which waits for the container to exit.
	go func() {
		// FIXME: It would be *much* better if the container '/wait'
		// API first acknowledged the request (perhaps by returning
		// response headers immediately?) then we could eliminate any
		// race condition between this request and the container
		// '/start' request made by the caller.
		exitStatus, err := dockerCli.Client().ContainerWait(ctx, containerID)
		if err != nil {
			logrus.Errorf("error waiting for container to exit: %v", err)
			cancel()
			return
		}

		// If we are talking to an older daemon, `AutoRemove` is not
		// supported. We need to fall back to the old behavior, which
		// is client-side removal.
		if waitRemove && versions.LessThan(dockerCli.Client().ClientVersion(), "1.25") {
			err = dockerCli.Client().ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{RemoveVolumes: true})
			if err != nil {
				logrus.Errorf("error removing container: %v", err)
				cancel()
				return
			}
		}

		waitChan <- int(exitStatus)
	}()

	resultChan := make(chan int)

	go func() {
		exitCode := 125

		// Must always send an exit code or the caller will block.
		defer func() {
			resultChan <- exitCode
		}()

		select {
		case waitStatus := <-waitChan:
			exitCode = waitStatus
		case <-waitCtx.Done():
			logrus.Errorf("unable to wait for container exit status: %v", waitCtx.Err())
		}
	}()

	return resultChan
}

// getExitCode performs an inspect on the container. It returns
// the running state and the exit code.
func getExitCode(ctx context.Context, dockerCli *command.DockerCli, containerID string) (bool, int, error) {
	c, err := dockerCli.Client().ContainerInspect(ctx, containerID)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if !clientapi.IsErrConnectionFailed(err) {
			return false, -1, err
		}
		return false, -1, nil
	}
	return c.State.Running, c.State.ExitCode, nil
}

func parallelOperation(ctx context.Context, containers []string, op func(ctx context.Context, container string) error) chan error {
	if len(containers) == 0 {
		return nil
	}
	const defaultParallel int = 50
	sem := make(chan struct{}, defaultParallel)
	errChan := make(chan error)

	// make sure result is printed in correct order
	output := map[string]chan error{}
	for _, c := range containers {
		output[c] = make(chan error, 1)
	}
	go func() {
		for _, c := range containers {
			err := <-output[c]
			errChan <- err
		}
	}()

	go func() {
		for _, c := range containers {
			sem <- struct{}{} // Wait for active queue sem to drain.
			go func(container string) {
				output[container] <- op(ctx, container)
				<-sem
			}(c)
		}
	}()
	return errChan
}
