package container

import (
	"fmt"
	"strconv"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/system"
	clientapi "github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
)

func waitExitOrRemoved(dockerCli *client.DockerCli, ctx context.Context, containerID string, waitRemove bool) (chan int, error) {
	if len(containerID) == 0 {
		// containerID can never be empty
		panic("Internal Error: waitExitOrRemoved needs a containerID as parameter")
	}

	statusChan := make(chan int)
	exitCode := 125

	eventProcessor := func(e events.Message, err error) error {
		if err != nil {
			statusChan <- exitCode
			return fmt.Errorf("failed to decode event: %v", err)
		}

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
			// stop the loop processing
			return fmt.Errorf("done")
		}

		return nil
	}

	// Get events via Events API
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("container", containerID)
	options := types.EventsOptions{
		Filters: f,
	}
	resBody, err := dockerCli.Client().Events(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("can't get events from daemon: %v", err)
	}

	go system.DecodeEvents(resBody, eventProcessor)

	return statusChan, nil
}

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
