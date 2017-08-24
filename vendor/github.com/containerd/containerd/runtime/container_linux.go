package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/containerd/specs"
	ocs "github.com/opencontainers/runtime-spec/specs-go"
)

func findCgroupMountpointAndRoot(pid int, subsystem string) (string, string, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/mountinfo", pid))
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		txt := scanner.Text()
		fields := strings.Split(txt, " ")
		for _, opt := range strings.Split(fields[len(fields)-1], ",") {
			if opt == subsystem {
				return fields[4], fields[3], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	return "", "", fmt.Errorf("cgroup path for %s not found", subsystem)
}

func parseCgroupFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	cgroups := make(map[string]string)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		text := s.Text()
		parts := strings.Split(text, ":")

		for _, subs := range strings.Split(parts[1], ",") {
			cgroups[subs] = parts[2]
		}
	}
	return cgroups, nil
}

func (c *container) OOM() (OOM, error) {
	p := c.processes[InitProcessID]
	if p == nil {
		return nil, fmt.Errorf("no init process found")
	}

	mountpoint, hostRoot, err := findCgroupMountpointAndRoot(os.Getpid(), "memory")
	if err != nil {
		return nil, err
	}

	cgroups, err := parseCgroupFile(fmt.Sprintf("/proc/%d/cgroup", p.pid))
	if err != nil {
		return nil, err
	}

	root, ok := cgroups["memory"]
	if !ok {
		return nil, fmt.Errorf("no memory cgroup for container %s", c.ID())
	}

	// Take care of the case were we're running inside a container
	// ourself
	root = strings.TrimPrefix(root, hostRoot)

	return c.getMemoryEventFD(filepath.Join(mountpoint, root))
}

func (c *container) Pids() ([]int, error) {
	var pids []int
	args := c.runtimeArgs
	args = append(args, "ps", "--format=json", c.id)
	out, err := exec.Command(c.runtime, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %q", err.Error(), out)
	}
	if err := json.Unmarshal(out, &pids); err != nil {
		return nil, err
	}
	return pids, nil
}

func u64Ptr(i uint64) *uint64 { return &i }
func i64Ptr(i int64) *int64   { return &i }

func (c *container) UpdateResources(r *Resource) error {
	sr := ocs.LinuxResources{
		Memory: &ocs.LinuxMemory{
			Limit:       i64Ptr(r.Memory),
			Reservation: i64Ptr(r.MemoryReservation),
			Swap:        i64Ptr(r.MemorySwap),
			Kernel:      i64Ptr(r.KernelMemory),
			KernelTCP:   i64Ptr(r.KernelTCPMemory),
		},
		CPU: &ocs.LinuxCPU{
			Shares:          u64Ptr(uint64(r.CPUShares)),
			Quota:           i64Ptr(int64(r.CPUQuota)),
			Period:          u64Ptr(uint64(r.CPUPeriod)),
			Cpus:            r.CpusetCpus,
			Mems:            r.CpusetMems,
			RealtimePeriod:  u64Ptr(uint64(r.CPURealtimePeriod)),
			RealtimeRuntime: i64Ptr(int64(r.CPURealtimdRuntime)),
		},
		BlockIO: &ocs.LinuxBlockIO{
			Weight: &r.BlkioWeight,
		},
		Pids: &ocs.LinuxPids{
			Limit: r.PidsLimit,
		},
	}

	srStr := bytes.NewBuffer(nil)
	if err := json.NewEncoder(srStr).Encode(&sr); err != nil {
		return err
	}

	args := c.runtimeArgs
	args = append(args, "update", "-r", "-", c.id)
	cmd := exec.Command(c.runtime, args...)
	cmd.Stdin = srStr
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(string(b))
	}
	return nil
}

func getRootIDs(s *specs.Spec) (int, int, error) {
	if s == nil {
		return 0, 0, nil
	}
	var hasUserns bool
	for _, ns := range s.Linux.Namespaces {
		if ns.Type == ocs.UserNamespace {
			hasUserns = true
			break
		}
	}
	if !hasUserns {
		return 0, 0, nil
	}
	uid := hostIDFromMap(0, s.Linux.UIDMappings)
	gid := hostIDFromMap(0, s.Linux.GIDMappings)
	return uid, gid, nil
}

func (c *container) getMemoryEventFD(root string) (*oom, error) {
	f, err := os.Open(filepath.Join(root, "memory.oom_control"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fd, _, serr := syscall.RawSyscall(syscall.SYS_EVENTFD2, 0, syscall.FD_CLOEXEC, 0)
	if serr != 0 {
		return nil, serr
	}
	if err := c.writeEventFD(root, int(f.Fd()), int(fd)); err != nil {
		syscall.Close(int(fd))
		return nil, err
	}
	return &oom{
		root:    root,
		id:      c.id,
		eventfd: int(fd),
	}, nil
}
