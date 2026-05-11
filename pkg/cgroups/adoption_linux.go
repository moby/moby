package cgroups

import (
	"fmt"
	"os"
	"strings"
)

// DeriveParentFromPid reads /proc/<pid>/cgroup and derives the appropriate
// cgroup parent path for containers created by this process.
//
// It attempts to extract the deepest `.slice` component from the cgroup path,
// which represents the systemd slice that should be used as the container's parent.
//
// For cgroup v2 (unified hierarchy), reads the single line prefixed with "0::".
// For cgroup v1, prioritizes the "name=systemd" controller.
//
// Returns an error if the cgroup file cannot be read or parsed.
func DeriveParentFromPid(pid int) (string, error) {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	return deriveParentFromCgroupFile(cgroupPath)
}

// deriveParentFromCgroupFile reads a cgroup file and extracts the parent cgroup slice.
// Separated from DeriveParentFromPid for testability.
func deriveParentFromCgroupFile(cgroupPath string) (string, error) {
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return "", fmt.Errorf("failed to read cgroup file: %w", err)
	}

	if len(data) == 0 {
		return "", fmt.Errorf("cgroup file is empty")
	}

	lines := strings.Split(string(data), "\n")

	var cgPath string

	// Parse cgroup file format:
	// - Cgroup v2: "0::/path/to/cgroup"
	// - Cgroup v1: "hierarchy-ID:controller-list:path"
	//
	// We prefer cgroup v2 unified hierarchy if present, otherwise fall back to
	// the systemd controller from v1.
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		hierarchyID := parts[0]
		controllers := parts[1]
		path := parts[2]

		// Cgroup v2 unified hierarchy
		if hierarchyID == "0" && controllers == "" {
			cgPath = path
			break
		}

		// Cgroup v1: prefer systemd controller
		if controllers == "name=systemd" {
			cgPath = path
			// Keep searching in case a v2 line appears later
		}

		// Cgroup v1: fallback to cpu controller if no systemd found yet
		if cgPath == "" && strings.Contains(controllers, "cpu") {
			cgPath = path
		}
	}

	if cgPath == "" {
		return "", fmt.Errorf("no valid cgroup path found in file")
	}

	// Return the full cgroup path so containers inherit the exact cgroup hierarchy
	// of their creator. This ensures that SLURM jobs, systemd scopes, and other
	// cgroup hierarchies are preserved.
	//
	// For example:
	//   - "/system.slice/slurmstepd.scope/job_123/step_0/user/task_0" -> "system.slice/slurmstepd.scope/job_123/step_0/user/task_0"
	//   - "/user.slice/user-1000.slice/session-1.scope" -> "user.slice/user-1000.slice/session-1.scope"
	//
	// This ensures containers are properly accounted under the creator's resource limits.
	if cgPath == "/" || cgPath == "" {
		return "", fmt.Errorf("cannot derive cgroup parent from root cgroup")
	}

	// Return the full path without leading slash
	return strings.TrimPrefix(cgPath, "/"), nil
}
