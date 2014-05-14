package fs

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type cpusetGroup struct {
}

func (s *cpusetGroup) Set(d *data) error {
	// we don't want to join this cgroup unless it is specified
	if d.c.CpusetCpus != "" {
		dir, err := d.path("cpuset")
		if err != nil {
			return err
		}
		if err := s.ensureParent(dir); err != nil {
			return err
		}

		// because we are not using d.join we need to place the pid into the procs file
		// unlike the other subsystems
		if err := writeFile(dir, "cgroup.procs", strconv.Itoa(d.pid)); err != nil {
			return err
		}
		if err := writeFile(dir, "cpuset.cpus", d.c.CpusetCpus); err != nil {
			return err
		}
	}
	return nil
}

func (s *cpusetGroup) Remove(d *data) error {
	return removePath(d.path("cpuset"))
}

func (s *cpusetGroup) Stats(d *data) (map[string]float64, error) {
	return nil, ErrNotSupportStat
}

func (s *cpusetGroup) getSubsystemSettings(parent string) (cpus []byte, mems []byte, err error) {
	if cpus, err = ioutil.ReadFile(filepath.Join(parent, "cpuset.cpus")); err != nil {
		return
	}
	if mems, err = ioutil.ReadFile(filepath.Join(parent, "cpuset.mems")); err != nil {
		return
	}
	return cpus, mems, nil
}

// ensureParent ensures that the parent directory of current is created
// with the proper cpus and mems files copied from it's parent if the values
// are a file with a new line char
func (s *cpusetGroup) ensureParent(current string) error {
	parent := filepath.Dir(current)

	if _, err := os.Stat(parent); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		if err := s.ensureParent(parent); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(current, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	return s.copyIfNeeded(current, parent)
}

// copyIfNeeded copies the cpuset.cpus and cpuset.mems from the parent
// directory to the current directory if the file's contents are 0
func (s *cpusetGroup) copyIfNeeded(current, parent string) error {
	var (
		err                      error
		currentCpus, currentMems []byte
		parentCpus, parentMems   []byte
	)

	if currentCpus, currentMems, err = s.getSubsystemSettings(current); err != nil {
		return err
	}
	if parentCpus, parentMems, err = s.getSubsystemSettings(parent); err != nil {
		return err
	}

	if s.isEmpty(currentCpus) {
		if err := writeFile(current, "cpuset.cpus", string(parentCpus)); err != nil {
			return err
		}
	}
	if s.isEmpty(currentMems) {
		if err := writeFile(current, "cpuset.mems", string(parentMems)); err != nil {
			return err
		}
	}
	return nil
}

func (s *cpusetGroup) isEmpty(b []byte) bool {
	return len(bytes.Trim(b, "\n")) == 0
}
