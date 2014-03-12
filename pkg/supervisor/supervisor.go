package supervisor

import (
	"errors"
	"os"
	"sync"
	"time"
)

var (
	ErrProcessAlreadyExists = errors.New("process already exists for given name")
)

type (
	// ExitCallback is called with the exit code and any error encountered by the run
	ExitCallback func(int, error)
	Processes    map[string]*Process

	Supervisor struct {
		sync.Mutex
		group sync.WaitGroup

		processes Processes
	}
)

// New creates a new process supervisor
func New() *Supervisor {
	return &Supervisor{
		processes: Processes{},
		group:     sync.WaitGroup{},
	}
}

// Start a new processes under supervision
//
// name should be unique for all processes
// args should be presented as argv[0], args[1:]...
func (s *Supervisor) Start(name string, attachStd bool, env []string, callback ExitCallback, args ...string) error {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.processes[name]; exists {
		return ErrProcessAlreadyExists
	}
	process := NewProcess(args, env, attachStd)

	if err := process.Start(); err != nil {
		return err
	}

	s.processes[name] = process
	s.group.Add(1)
	go func() {
		var (
			err  = process.Wait()
			exit = process.ExitCode()
		)
		if callback != nil {
			callback(exit, err)
		}
		s.Lock()
		delete(s.processes, name)
		s.Unlock()
		s.group.Done()
	}()
	return nil
}

// Forward forwards signals to the child processes
func (s *Supervisor) Forward(sig os.Signal) error {
	s.Lock()
	defer s.Unlock()

	var err error
	for _, process := range s.processes {
		if perr := process.Signal(sig); err == nil {
			err = perr
		}
	}
	return err
}

// Wait will wait on all processes being supervised
func (s *Supervisor) Wait() error {
	s.group.Wait()
	return nil
}

func (s *Supervisor) Reap(sig os.Signal, timeout time.Duration) error {
	err := s.Forward(sig)
	c := make(chan struct{})

	go func() {
		s.Wait()
		c <- struct{}{}
	}()
	t := time.NewTicker(timeout)
	defer t.Stop()

	select {
	case <-c:
		return err
	case <-t.C:
		for _, p := range s.processes {
			if perr := p.Reap(); err == nil {
				err = perr
			}
		}
	}
	return err
}
