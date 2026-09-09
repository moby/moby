//go:build linux

package cgrouputil

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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

// IsScopePath reports whether a cgroup v2 path is inside a systemd scope
// (any path segment ending in ".scope", e.g.
// "/user.slice/user-1000.slice/session-3.scope" or
// "/system.slice/slurmstepd.scope/job_123").
//
// Scopes are leaf cgroups that hold processes and cannot have children, so
// they cannot be used as a CgroupParent for a new container under the
// systemd cgroup driver (which requires a "xxx.slice" parent). Placing a
// container under a scope already managed by another manager (e.g. Slurm)
// would also require a second cgroup manager on the same scope, which stock
// runc refuses via its eBPF device filter; that part is driver-independent,
// so scopes are rejected regardless of cgroup driver. Callers should reject
// scope paths fail-closed with an actionable error instead of passing them
// through to the generic ".slice" validation or to runc.
func IsScopePath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.HasSuffix(seg, ".scope") {
			return true
		}
	}
	return false
}

// VerifyPIDOwner mitigates PID-reuse (TOCTOU) between SO_PEERCRED collection
// (at connection time) and /proc/<pid>/cgroup reads (at create time): it
// checks that the UID owning /proc/<pid> still matches the peer credential
// UID. If the original client exited and the PID was recycled by another
// user, the UIDs differ and an error is returned.
//
// SO_PEERCRED reports the effective UID at connect time, so any of the
// real/effective/saved/filesystem UIDs in /proc/<pid>/status matching is
// accepted (avoids false rejections for setuid/sudo clients).
//
// This does not close the reuse window entirely (same-UID reuse is still
// possible, as is reuse after this check returns); a full fix would pass
// credentials per-request instead of per connection. It does prevent the
// worst case of attributing one user's container to another user's cgroup.
func VerifyPIDOwner(pid int32, uid uint32) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return fmt.Errorf("malformed Uid line for pid %d", pid)
		}
		for _, s := range fields[1:5] {
			v, err := strconv.ParseUint(s, 10, 32)
			if err != nil {
				return fmt.Errorf("invalid uid for pid %d: %w", pid, err)
			}
			if uint32(v) == uid {
				return nil
			}
		}
		return fmt.Errorf("process %d is now owned by uids %q, not peer uid %d (pid may have been reused)", pid, strings.Join(fields[1:5], ","), uid)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("uid not found for pid %d", pid)
}
