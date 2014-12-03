package cgroups

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/mount"
)

// https://www.kernel.org/doc/Documentation/cgroups/cgroups.txt
func FindCgroupMountpoint(subsystem string) (string, error) {
	mounts, err := mount.GetMounts()
	if err != nil {
		return "", err
	}

	for _, mount := range mounts {
		if mount.Fstype == "cgroup" {
			for _, opt := range strings.Split(mount.VfsOpts, ",") {
				if opt == subsystem {
					return mount.Mountpoint, nil
				}
			}
		}
	}

	return "", NewNotFoundError(subsystem)
}

type Mount struct {
	Mountpoint string
	Subsystems []string
}

func (m Mount) GetThisCgroupDir() (string, error) {
	if len(m.Subsystems) == 0 {
		return "", fmt.Errorf("no subsystem for mount")
	}

	return GetThisCgroupDir(m.Subsystems[0])
}

func GetCgroupMounts() ([]Mount, error) {
	mounts, err := mount.GetMounts()
	if err != nil {
		return nil, err
	}

	all, err := GetAllSubsystems()
	if err != nil {
		return nil, err
	}

	allMap := make(map[string]bool)
	for _, s := range all {
		allMap[s] = true
	}

	res := []Mount{}
	for _, mount := range mounts {
		if mount.Fstype == "cgroup" {
			m := Mount{Mountpoint: mount.Mountpoint}

			for _, opt := range strings.Split(mount.VfsOpts, ",") {
				if strings.HasPrefix(opt, "name=") {
					m.Subsystems = append(m.Subsystems, opt)
				}
				if allMap[opt] {
					m.Subsystems = append(m.Subsystems, opt)
				}
			}
			res = append(res, m)
		}
	}
	return res, nil
}

// Returns all the cgroup subsystems supported by the kernel
func GetAllSubsystems() ([]string, error) {
	f, err := os.Open("/proc/cgroups")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	subsystems := []string{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}
		text := s.Text()
		if text[0] != '#' {
			parts := strings.Fields(text)
			if len(parts) >= 4 && parts[3] != "0" {
				subsystems = append(subsystems, parts[0])
			}
		}
	}
	return subsystems, nil
}

// Returns the relative path to the cgroup docker is running in.
func GetThisCgroupDir(subsystem string) (string, error) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	defer f.Close()

	return ParseCgroupFile(subsystem, f)
}

func GetInitCgroupDir(subsystem string) (string, error) {
	f, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return "", err
	}
	defer f.Close()

	return ParseCgroupFile(subsystem, f)
}

func ReadProcsFile(dir string) ([]int, error) {
	f, err := os.Open(filepath.Join(dir, "cgroup.procs"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		s   = bufio.NewScanner(f)
		out = []int{}
	)

	for s.Scan() {
		if t := s.Text(); t != "" {
			pid, err := strconv.Atoi(t)
			if err != nil {
				return nil, err
			}
			out = append(out, pid)
		}
	}
	return out, nil
}

func ParseCgroupFile(subsystem string, r io.Reader) (string, error) {
	s := bufio.NewScanner(r)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return "", err
		}

		text := s.Text()
		parts := strings.Split(text, ":")

		for _, subs := range strings.Split(parts[1], ",") {
			if subs == subsystem {
				return parts[2], nil
			}
		}
	}

	return "", NewNotFoundError(subsystem)
}

func pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func EnterPid(cgroupPaths map[string]string, pid int) error {
	for _, path := range cgroupPaths {
		if pathExists(path) {
			if err := ioutil.WriteFile(filepath.Join(path, "cgroup.procs"),
				[]byte(strconv.Itoa(pid)), 0700); err != nil {
				return err
			}
		}
	}
	return nil
}

// RemovePaths iterates over the provided paths removing them.
// If an error is encountered the removal proceeds and the first error is
// returned to ensure a partial removal is not possible.
func RemovePaths(paths map[string]string) (err error) {
	for _, path := range paths {
		if rerr := os.RemoveAll(path); err == nil {
			err = rerr
		}
	}
	return err
}
