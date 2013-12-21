package cgroups

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/mount"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	return "", fmt.Errorf("cgroup mountpoint not found for %s", subsystem)
}

// Returns the relative path to the cgroup docker is running in.
func getThisCgroupDir(subsystem string) (string, error) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	defer f.Close()

	return parseCgroupFile(subsystem, f)
}

func parseCgroupFile(subsystem string, r io.Reader) (string, error) {
	s := bufio.NewScanner(r)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return "", err
		}
		text := s.Text()
		parts := strings.Split(text, ":")
		if parts[1] == subsystem {
			return parts[2], nil
		}
	}
	return "", fmt.Errorf("cgroup '%s' not found in /proc/self/cgroup", subsystem)
}

// Returns a list of pids for the given container.
func GetPidsForContainer(id string) ([]int, error) {
	pids := []int{}

	// memory is chosen randomly, any cgroup used by docker works
	subsystem := "memory"

	cgroupRoot, err := FindCgroupMountpoint(subsystem)
	if err != nil {
		return pids, err
	}

	cgroupDir, err := getThisCgroupDir(subsystem)
	if err != nil {
		return pids, err
	}

	filename := filepath.Join(cgroupRoot, cgroupDir, id, "tasks")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// With more recent lxc versions use, cgroup will be in lxc/
		filename = filepath.Join(cgroupRoot, cgroupDir, "lxc", id, "tasks")
	}

	output, err := ioutil.ReadFile(filename)
	if err != nil {
		return pids, err
	}
	for _, p := range strings.Split(string(output), "\n") {
		if len(p) == 0 {
			continue
		}
		pid, err := strconv.Atoi(p)
		if err != nil {
			return pids, fmt.Errorf("Invalid pid '%s': %s", p, err)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}
