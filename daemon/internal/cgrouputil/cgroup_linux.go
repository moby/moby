//go:build linux

package cgrouputil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// GetClientCgroup reads /proc/<pid>/cgroup and returns the cgroup path
// for the client process. It handles both cgroups v1 and v2.
//
// For cgroups v2 (unified), it returns the path from the line "0::/path".
// For cgroups v1, it prefers the "memory" controller, then "cpu", then the
// longest path. If only "/" is found, it returns "/".
func GetClientCgroup(pid int32) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("invalid pid %d", pid)
	}
	path := fmt.Sprintf("/proc/%d/cgroup", pid)
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var unifiedPath string
	cgroups := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Format: hierarchy_id:controllers:path  or 0::/path for v2
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		controllers := parts[1]
		cgroupPath := parts[2]
		if cgroupPath == "" {
			continue
		}
		// cgroups v2 unified entry
		if parts[0] == "0" && controllers == "" {
			unifiedPath = cgroupPath
			continue
		}
		// cgroups v1: controllers is comma-separated
		for _, c := range strings.Split(controllers, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				// Keep the longest path for a given controller (most specific)
				if existing, ok := cgroups[c]; !ok || len(cgroupPath) > len(existing) {
					cgroups[c] = cgroupPath
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if unifiedPath != "" {
		return unifiedPath, nil
	}
	// Prefer memory, then cpu, then systemd, then pids, then longest
	for _, ctrl := range []string{"memory", "cpu", "cpu,cpuacct", "pids", "systemd"} {
		if p, ok := cgroups[ctrl]; ok && p != "" {
			return p, nil
		}
	}
	// Fallback to longest path among all controllers
	var longest string
	for _, p := range cgroups {
		if len(p) > len(longest) {
			longest = p
		}
	}
	if longest != "" {
		return longest, nil
	}
	return "", fmt.Errorf("no cgroup path found for pid %d", pid)
}
