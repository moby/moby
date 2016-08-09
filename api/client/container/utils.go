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

	var exitCode int
	statusChan := make(chan int)
	exitChan := make(chan struct{})
	detachChan := make(chan struct{})
	destroyChan := make(chan struct{})
	eventsErr := make(chan error)

	// Start watch events
	eh := system.InitEventHandler()
	eh.Handle("die", func(e events.Message) {
		if len(e.Actor.Attributes) > 0 {
			for k, v := range e.Actor.Attributes {
				if k == "exitCode" {
					var err error
					if exitCode, err = strconv.Atoi(v); err != nil {
						logrus.Errorf("Can't convert %q to int: %v", v, err)
					}
					close(exitChan)
					break
				}
			}
		}
	})

	eh.Handle("detach", func(e events.Message) {
		exitCode = 0
		close(detachChan)
	})
	eh.Handle("destroy", func(e events.Message) {
		close(destroyChan)
	})

	eventChan := make(chan events.Message)
	go eh.Watch(eventChan)

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

	go func() {
		eventsErr <- system.DecodeEvents(resBody, func(event events.Message, err error) error {
			if err != nil {
				return fmt.Errorf("decode events error: %v", err)
			}
			eventChan <- event
			return nil
		})
		close(eventChan)
	}()

	go func() {
		var waitErr error
		if waitRemove {
			select {
			case <-destroyChan:
				// keep exitcode and return
			case <-detachChan:
				exitCode = 0
			case waitErr = <-eventsErr:
				exitCode = 125
			}
		} else {
			select {
			case <-exitChan:
				// keep exitcode and return
			case <-detachChan:
				exitCode = 0
			case waitErr = <-eventsErr:
				exitCode = 125
			}
		}
		if waitErr != nil {
			logrus.Errorf("%v", waitErr)
		}
		statusChan <- exitCode

		resBody.Close()
	}()
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
