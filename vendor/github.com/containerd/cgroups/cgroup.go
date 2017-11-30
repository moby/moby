package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// New returns a new control via the cgroup cgroups interface
func New(hierarchy Hierarchy, path Path, resources *specs.LinuxResources) (Cgroup, error) {
	subsystems, err := hierarchy()
	if err != nil {
		return nil, err
	}
	for _, s := range subsystems {
		if err := initializeSubsystem(s, path, resources); err != nil {
			return nil, err
		}
	}
	return &cgroup{
		path:       path,
		subsystems: subsystems,
	}, nil
}

// Load will load an existing cgroup and allow it to be controlled
func Load(hierarchy Hierarchy, path Path) (Cgroup, error) {
	subsystems, err := hierarchy()
	if err != nil {
		return nil, err
	}
	// check the the subsystems still exist
	for _, s := range pathers(subsystems) {
		p, err := path(s.Name())
		if err != nil {
			if os.IsNotExist(errors.Cause(err)) {
				return nil, ErrCgroupDeleted
			}
			return nil, err
		}
		if _, err := os.Lstat(s.Path(p)); err != nil {
			if os.IsNotExist(err) {
				return nil, ErrCgroupDeleted
			}
			return nil, err
		}
	}
	return &cgroup{
		path:       path,
		subsystems: subsystems,
	}, nil
}

type cgroup struct {
	path Path

	subsystems []Subsystem
	mu         sync.Mutex
	err        error
}

// New returns a new sub cgroup
func (c *cgroup) New(name string, resources *specs.LinuxResources) (Cgroup, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	path := subPath(c.path, name)
	for _, s := range c.subsystems {
		if err := initializeSubsystem(s, path, resources); err != nil {
			return nil, err
		}
	}
	return &cgroup{
		path:       path,
		subsystems: c.subsystems,
	}, nil
}

// Subsystems returns all the subsystems that are currently being
// consumed by the group
func (c *cgroup) Subsystems() []Subsystem {
	return c.subsystems
}

// Add moves the provided process into the new cgroup
func (c *cgroup) Add(process Process) error {
	if process.Pid <= 0 {
		return ErrInvalidPid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	return c.add(process)
}

func (c *cgroup) add(process Process) error {
	for _, s := range pathers(c.subsystems) {
		p, err := c.path(s.Name())
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(
			filepath.Join(s.Path(p), cgroupProcs),
			[]byte(strconv.Itoa(process.Pid)),
			defaultFilePerm,
		); err != nil {
			return err
		}
	}
	return nil
}

// Delete will remove the control group from each of the subsystems registered
func (c *cgroup) Delete() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	var errors []string
	for _, s := range c.subsystems {
		if d, ok := s.(deleter); ok {
			sp, err := c.path(s.Name())
			if err != nil {
				return err
			}
			if err := d.Delete(sp); err != nil {
				errors = append(errors, string(s.Name()))
			}
			continue
		}
		if p, ok := s.(pather); ok {
			sp, err := c.path(s.Name())
			if err != nil {
				return err
			}
			path := p.Path(sp)
			if err := remove(path); err != nil {
				errors = append(errors, path)
			}
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("cgroups: unable to remove paths %s", strings.Join(errors, ", "))
	}
	c.err = ErrCgroupDeleted
	return nil
}

// Stat returns the current metrics for the cgroup
func (c *cgroup) Stat(handlers ...ErrorHandler) (*Metrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	if len(handlers) == 0 {
		handlers = append(handlers, errPassthrough)
	}
	var (
		stats = &Metrics{
			CPU: &CPUStat{
				Throttling: &Throttle{},
				Usage:      &CPUUsage{},
			},
		}
		wg   = &sync.WaitGroup{}
		errs = make(chan error, len(c.subsystems))
	)
	for _, s := range c.subsystems {
		if ss, ok := s.(stater); ok {
			sp, err := c.path(s.Name())
			if err != nil {
				return nil, err
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := ss.Stat(sp, stats); err != nil {
					for _, eh := range handlers {
						if herr := eh(err); herr != nil {
							errs <- herr
						}
					}
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return nil, err
	}
	return stats, nil
}

// Update updates the cgroup with the new resource values provided
//
// Be prepared to handle EBUSY when trying to update a cgroup with
// live processes and other operations like Stats being performed at the
// same time
func (c *cgroup) Update(resources *specs.LinuxResources) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	for _, s := range c.subsystems {
		if u, ok := s.(updater); ok {
			sp, err := c.path(s.Name())
			if err != nil {
				return err
			}
			if err := u.Update(sp, resources); err != nil {
				return err
			}
		}
	}
	return nil
}

// Processes returns the processes running inside the cgroup along
// with the subsystem used, pid, and path
func (c *cgroup) Processes(subsystem Name, recursive bool) ([]Process, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return c.processes(subsystem, recursive)
}

func (c *cgroup) processes(subsystem Name, recursive bool) ([]Process, error) {
	s := c.getSubsystem(subsystem)
	sp, err := c.path(subsystem)
	if err != nil {
		return nil, err
	}
	path := s.(pather).Path(sp)
	var processes []Process
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !recursive && info.IsDir() {
			if p == path {
				return nil
			}
			return filepath.SkipDir
		}
		dir, name := filepath.Split(p)
		if name != cgroupProcs {
			return nil
		}
		procs, err := readPids(dir, subsystem)
		if err != nil {
			return err
		}
		processes = append(processes, procs...)
		return nil
	})
	return processes, err
}

// Freeze freezes the entire cgroup and all the processes inside it
func (c *cgroup) Freeze() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	s := c.getSubsystem(Freezer)
	if s == nil {
		return ErrFreezerNotSupported
	}
	sp, err := c.path(Freezer)
	if err != nil {
		return err
	}
	return s.(*freezerController).Freeze(sp)
}

// Thaw thaws out the cgroup and all the processes inside it
func (c *cgroup) Thaw() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	s := c.getSubsystem(Freezer)
	if s == nil {
		return ErrFreezerNotSupported
	}
	sp, err := c.path(Freezer)
	if err != nil {
		return err
	}
	return s.(*freezerController).Thaw(sp)
}

// OOMEventFD returns the memory cgroup's out of memory event fd that triggers
// when processes inside the cgroup receive an oom event. Returns
// ErrMemoryNotSupported if memory cgroups is not supported.
func (c *cgroup) OOMEventFD() (uintptr, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return 0, c.err
	}
	s := c.getSubsystem(Memory)
	if s == nil {
		return 0, ErrMemoryNotSupported
	}
	sp, err := c.path(Memory)
	if err != nil {
		return 0, err
	}
	return s.(*memoryController).OOMEventFD(sp)
}

// State returns the state of the cgroup and its processes
func (c *cgroup) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkExists()
	if c.err != nil && c.err == ErrCgroupDeleted {
		return Deleted
	}
	s := c.getSubsystem(Freezer)
	if s == nil {
		return Thawed
	}
	sp, err := c.path(Freezer)
	if err != nil {
		return Unknown
	}
	state, err := s.(*freezerController).state(sp)
	if err != nil {
		return Unknown
	}
	return state
}

// MoveTo does a recursive move subsystem by subsystem of all the processes
// inside the group
func (c *cgroup) MoveTo(destination Cgroup) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	for _, s := range c.subsystems {
		processes, err := c.processes(s.Name(), true)
		if err != nil {
			return err
		}
		for _, p := range processes {
			if err := destination.Add(p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *cgroup) getSubsystem(n Name) Subsystem {
	for _, s := range c.subsystems {
		if s.Name() == n {
			return s
		}
	}
	return nil
}

func (c *cgroup) checkExists() {
	for _, s := range pathers(c.subsystems) {
		p, err := c.path(s.Name())
		if err != nil {
			return
		}
		if _, err := os.Lstat(s.Path(p)); err != nil {
			if os.IsNotExist(err) {
				c.err = ErrCgroupDeleted
				return
			}
		}
	}
}
