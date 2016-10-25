package container

import (
	"strconv"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/cli/command"
	clientapi "github.com/docker/docker/client"
)

func waitExitOrRemoved(dockerCli *command.DockerCli, ctx context.Context, containerID string, waitRemove bool) chan int {
	if len(containerID) == 0 {
		// containerID can never be empty
		panic("Internal Error: waitExitOrRemoved needs a containerID as parameter")
	}

	statusChan := make(chan int)
	exitCode := 125

	eventProcessor := func(e events.Message) bool {

		stopProcessing := false
		switch e.Status {
		case "die":
			if v, ok := e.Actor.Attributes["exitCode"]; ok {
				code, cerr := strconv.Atoi(v)
				if cerr != nil {
					logrus.Errorf("failed to convert exitcode '%q' to int: %v", v, cerr)
				} else {
					exitCode = code
				}
			}
			if !waitRemove {
				stopProcessing = true
			}
		case "detach":
			exitCode = 0
			stopProcessing = true
		case "destroy":
			stopProcessing = true
		}

		if stopProcessing {
			statusChan <- exitCode
			return true
		}

		return false
	}

	// Get events via Events API
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", containerID)
	options := types.EventsOptions{
		Filters: f,
	}

	eventCtx, cancel := context.WithCancel(ctx)
	eventq, errq := dockerCli.Client().Events(eventCtx, options)

	go func() {
		defer cancel()

		for {
			select {
			case evt := <-eventq:
				if eventProcessor(evt) {
					return
				}

			case err := <-errq:
				logrus.Errorf("error getting events from daemon: %v", err)
				statusChan <- exitCode
				return
			}
		}
	}()

	return statusChan
}

// getExitCode performs an inspect on the container. It returns
// the running state and the exit code.
func getExitCode(dockerCli *command.DockerCli, ctx context.Context, containerID string) (bool, int, error) {
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
