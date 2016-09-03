package container

import (
	"sync/atomic"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestStateRunStop(c *check.C) {
	ns := NewState()
	for i := 1; i < 3; i++ { // full lifecycle two times
		ns.Lock()
		ns.SetRunning(i+100, false)
		ns.Unlock()

		if !ns.IsRunning() {
			c.Fatal("State not running")
		}
		if ns.Pid != i+100 {
			c.Fatalf("Pid %v, expected %v", ns.Pid, i+100)
		}
		if ns.ExitCode() != 0 {
			c.Fatalf("ExitCode %v, expected 0", ns.ExitCode())
		}

		stopped := make(chan struct{})
		var exit int64
		go func() {
			exitCode, _ := ns.WaitStop(-1 * time.Second)
			atomic.StoreInt64(&exit, int64(exitCode))
			close(stopped)
		}()
		ns.SetStoppedLocking(&ExitStatus{ExitCode: i})
		if ns.IsRunning() {
			c.Fatal("State is running")
		}
		if ns.ExitCode() != i {
			c.Fatalf("ExitCode %v, expected %v", ns.ExitCode(), i)
		}
		if ns.Pid != 0 {
			c.Fatalf("Pid %v, expected 0", ns.Pid)
		}
		select {
		case <-time.After(100 * time.Millisecond):
			c.Fatal("Stop callback doesn't fire in 100 milliseconds")
		case <-stopped:
			c.Log("Stop callback fired")
		}
		exitCode := int(atomic.LoadInt64(&exit))
		if exitCode != i {
			c.Fatalf("ExitCode %v, expected %v", exitCode, i)
		}
		if exitCode, err := ns.WaitStop(-1 * time.Second); err != nil || exitCode != i {
			c.Fatalf("WaitStop returned exitCode: %v, err: %v, expected exitCode: %v, err: %v", exitCode, err, i, nil)
		}
	}
}

func (s *DockerSuite) TestStateTimeoutWait(c *check.C) {
	ns := NewState()
	stopped := make(chan struct{})
	go func() {
		ns.WaitStop(100 * time.Millisecond)
		close(stopped)
	}()
	select {
	case <-time.After(200 * time.Millisecond):
		c.Fatal("Stop callback doesn't fire in 200 milliseconds")
	case <-stopped:
		c.Log("Stop callback fired")
	}

	ns.SetStoppedLocking(&ExitStatus{ExitCode: 1})

	stopped = make(chan struct{})
	go func() {
		ns.WaitStop(100 * time.Millisecond)
		close(stopped)
	}()
	select {
	case <-time.After(200 * time.Millisecond):
		c.Fatal("Stop callback doesn't fire in 100 milliseconds")
	case <-stopped:
		c.Log("Stop callback fired")
	}

}
