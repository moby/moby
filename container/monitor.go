package container

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container/stream"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/restartmanager"
	"golang.org/x/net/context"
)

const (
	loggerCloseTimeout = 10 * time.Second
)

func newRunState() *runState {
	return &runState{
		streams:       stream.NewConfig(),
		attachContext: &attachContext{},
	}
}

type runState struct {
	mu             sync.Mutex
	attachContext  *attachContext
	healthMonitor  chan struct{}
	logDriver      logger.Logger
	logCopier      *logger.Copier
	restartManager restartmanager.RestartManager
	streams        *stream.Config
}

func (s *runState) openHealthMonitor() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.healthMonitor == nil {
		s.healthMonitor = make(chan struct{})
		return s.healthMonitor
	}
	return nil
}

func (s *runState) closeHealthMonitor() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.healthMonitor != nil {
		close(s.healthMonitor)
		s.healthMonitor = nil
	}
}

func (s *runState) resetLogging() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.logDriver != nil {
		if s.logCopier != nil {
			ctx, cancel := context.WithTimeout(context.Background(), loggerCloseTimeout)
			go func() {
				s.logCopier.Wait()
				cancel()
			}()
			<-ctx.Done()
		}
		s.logDriver.Close()
	}

	s.logCopier = nil
	s.logDriver = nil
}

func (s *runState) Streams() *stream.Config {
	s.mu.Lock()
	streams := s.streams
	s.mu.Unlock()
	return streams
}

// Reset puts a container into a state where it can be restarted again.
func (container *Container) Reset() {
	streams := container.Streams()

	if err := streams.CloseStreams(); err != nil {
		logrus.Errorf("%s: %s", container.ID, err)
	}

	// Re-create a brand new stdin pipe once the container exited
	if container.Config.OpenStdin {
		streams.NewInputPipes()
	}

	container.runState.resetLogging()

	if container.State.Health != nil {
		container.State.Health.Status = types.Unhealthy
	}
}
