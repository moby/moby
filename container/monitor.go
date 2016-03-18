package container

import (
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	loggerCloseTimeout = 10 * time.Second
)

// supervisor defines the interface that a supervisor must implement
type supervisor interface {
	// LogContainerEvent generates events related to a given container
	LogContainerEvent(*Container, string)
	// Cleanup ensures that the container is properly unmounted
	Cleanup(*Container)
	// StartLogging starts the logging driver for the container
	StartLogging(*Container) error
	// Run starts a container
	Run(c *Container) error
	// IsShuttingDown tells whether the supervisor is shutting down or not
	IsShuttingDown() bool
}

// Reset puts a container into a state where it can be restarted again.
func (container *Container) Reset(lock bool) {
	if lock {
		container.Lock()
		defer container.Unlock()
	}

	if err := container.CloseStreams(); err != nil {
		logrus.Errorf("%s: %s", container.ID, err)
	}

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		container.NewInputPipes()
	}

	if container.LogDriver != nil {
		if container.LogCopier != nil {
			exit := make(chan struct{})
			go func() {
				container.LogCopier.Wait()
				close(exit)
			}()
			select {
			case <-time.After(loggerCloseTimeout):
				logrus.Warnf("Logger didn't exit in time: logs may be truncated")
			case <-exit:
			}
		}
		container.LogDriver.Close()
		container.LogCopier = nil
		container.LogDriver = nil
	}
}
