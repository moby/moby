package restartmanager

import (
	"fmt"
	"sync"
	"time"

	"github.com/docker/engine-api/types/container"
)

const (
	backoffMultiplier = 2
	defaultTimeout    = 100 * time.Millisecond
)

// RestartManager defines object that controls container restarting rules.
type RestartManager interface {
	Cancel() error
	ShouldRestart(exitCode uint32) (bool, chan error, error)
}

type restartManager struct {
	sync.Mutex
	sync.Once
	policy       container.RestartPolicy
	failureCount int
	timeout      time.Duration
	active       bool
	cancel       chan struct{}
	canceled     bool
}

// New returns a new restartmanager based on a policy.
func New(policy container.RestartPolicy) RestartManager {
	return &restartManager{policy: policy, cancel: make(chan struct{})}
}

func (rm *restartManager) SetPolicy(policy container.RestartPolicy) {
	rm.Lock()
	rm.policy = policy
	rm.Unlock()
}

func (rm *restartManager) ShouldRestart(exitCode uint32) (bool, chan error, error) {
	rm.Lock()
	unlockOnExit := true
	defer func() {
		if unlockOnExit {
			rm.Unlock()
		}
	}()

	if rm.canceled {
		return false, nil, nil
	}

	if rm.active {
		return false, nil, fmt.Errorf("invalid call on active restartmanager")
	}

	if exitCode != 0 {
		rm.failureCount++
	} else {
		rm.failureCount = 0
	}

	if rm.timeout == 0 {
		rm.timeout = defaultTimeout
	} else {
		rm.timeout *= backoffMultiplier
	}

	var restart bool
	switch {
	case rm.policy.IsAlways(), rm.policy.IsUnlessStopped():
		restart = true
	case rm.policy.IsOnFailure():
		// the default value of 0 for MaximumRetryCount means that we will not enforce a maximum count
		if max := rm.policy.MaximumRetryCount; max == 0 || rm.failureCount <= max {
			restart = exitCode != 0
		}
	}

	if !restart {
		rm.active = false
		return false, nil, nil
	}

	unlockOnExit = false
	rm.active = true
	rm.Unlock()

	ch := make(chan error)
	go func() {
		select {
		case <-rm.cancel:
			ch <- fmt.Errorf("restartmanager canceled")
			close(ch)
		case <-time.After(rm.timeout):
			rm.Lock()
			close(ch)
			rm.active = false
			rm.Unlock()
		}
	}()

	return true, ch, nil
}

func (rm *restartManager) Cancel() error {
	rm.Do(func() {
		rm.Lock()
		rm.canceled = true
		close(rm.cancel)
		rm.Unlock()
	})
	return nil
}
