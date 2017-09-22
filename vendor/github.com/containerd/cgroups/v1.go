package cgroups

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// V1 returns all the groups in the default cgroups mountpoint in a single hierarchy
func V1() ([]Subsystem, error) {
	root, err := v1MountPoint()
	if err != nil {
		return nil, err
	}
	subsystems, err := defaults(root)
	if err != nil {
		return nil, err
	}
	var enabled []Subsystem
	for _, s := range pathers(subsystems) {
		// check and remove the default groups that do not exist
		if _, err := os.Lstat(s.Path("/")); err == nil {
			enabled = append(enabled, s)
		}
	}
	return enabled, nil
}

// v1MountPoint returns the mount point where the cgroup
// mountpoints are mounted in a single hiearchy
func v1MountPoint() (string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		var (
			text   = scanner.Text()
			fields = strings.Split(text, " ")
			// safe as mountinfo encodes mountpoints with spaces as \040.
			index               = strings.Index(text, " - ")
			postSeparatorFields = strings.Fields(text[index+3:])
			numPostFields       = len(postSeparatorFields)
		)
		// this is an error as we can't detect if the mount is for "cgroup"
		if numPostFields == 0 {
			return "", fmt.Errorf("Found no fields post '-' in %q", text)
		}
		if postSeparatorFields[0] == "cgroup" {
			// check that the mount is properly formated.
			if numPostFields < 3 {
				return "", fmt.Errorf("Error found less than 3 fields post '-' in %q", text)
			}
			return filepath.Dir(fields[4]), nil
		}
	}
	return "", ErrMountPointNotExist
}
