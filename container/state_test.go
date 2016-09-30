package container

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestStateRunStop(t *testing.T) {
	s := NewState()
	for i := 1; i < 3; i++ { // full lifecycle two times
		s.Lock()
		s.SetRunning(i+100, false)
		s.Unlock()

		if !s.IsRunning() {
			t.Fatal("State not running")
		}
		if s.Pid != i+100 {
			t.Fatalf("Pid %v, expected %v", s.Pid, i+100)
		}
		if s.ExitCode() != 0 {
			t.Fatalf("ExitCode %v, expected 0", s.ExitCode())
		}

		stopped := make(chan struct{})
		var exit int64
		go func() {
			exitCode, _ := s.WaitStop(-1 * time.Second)
			atomic.StoreInt64(&exit, int64(exitCode))
			close(stopped)
		}()
		s.Lock()
		s.SetStopped(&ExitStatus{ExitCode: i})
		s.Unlock()
		if s.IsRunning() {
			t.Fatal("State is running")
		}
		if s.ExitCode() != i {
			t.Fatalf("ExitCode %v, expected %v", s.ExitCode(), i)
		}
		if s.Pid != 0 {
			t.Fatalf("Pid %v, expected 0", s.Pid)
		}
		select {
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Stop callback doesn't fire in 100 milliseconds")
		case <-stopped:
			t.Log("Stop callback fired")
		}
		exitCode := int(atomic.LoadInt64(&exit))
		if exitCode != i {
			t.Fatalf("ExitCode %v, expected %v", exitCode, i)
		}
		if exitCode, err := s.WaitStop(-1 * time.Second); err != nil || exitCode != i {
			t.Fatalf("WaitStop returned exitCode: %v, err: %v, expected exitCode: %v, err: %v", exitCode, err, i, nil)
		}
	}
}

func TestStateTimeoutWait(t *testing.T) {
	s := NewState()
	stopped := make(chan struct{})
	go func() {
		s.WaitStop(100 * time.Millisecond)
		close(stopped)
	}()
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop callback doesn't fire in 200 milliseconds")
	case <-stopped:
		t.Log("Stop callback fired")
	}

	s.Lock()
	s.SetStopped(&ExitStatus{ExitCode: 1})
	s.Unlock()

	stopped = make(chan struct{})
	go func() {
		s.WaitStop(100 * time.Millisecond)
		close(stopped)
	}()
	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Stop callback doesn't fire in 100 milliseconds")
	case <-stopped:
		t.Log("Stop callback fired")
	}

}
