//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	statsV1 "github.com/containerd/cgroups/v3/cgroup1/stats"
	statsV2 "github.com/containerd/cgroups/v3/cgroup2/stats"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/pkg/errors"
)

func copyBlkioEntry(entries []*statsV1.BlkIOEntry) []types.BlkioStatEntry {
	out := make([]types.BlkioStatEntry, len(entries))
	for i, re := range entries {
		out[i] = types.BlkioStatEntry{
			Major: re.Major,
			Minor: re.Minor,
			Op:    re.Op,
			Value: re.Value,
		}
	}
	return out
}

func (daemon *Daemon) stats(c *container.Container) (*types.StatsJSON, error) {
	c.Lock()
	task, err := c.GetRunningTask()
	c.Unlock()
	if err != nil {
		return nil, err
	}
	cs, err := task.Stats(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "container not found") {
			return nil, containerNotFound(c.ID)
		}
		return nil, err
	}
	s := &types.StatsJSON{}
	s.Read = cs.Read
	stats := cs.Metrics
	switch t := stats.(type) {
	case *statsV1.Metrics:
		return daemon.statsV1(s, t)
	case *statsV2.Metrics:
		return daemon.statsV2(s, t)
	default:
		return nil, errors.Errorf("unexpected type of metrics %+v", t)
	}
}

func (daemon *Daemon) statsV1(s *types.StatsJSON, stats *statsV1.Metrics) (*types.StatsJSON, error) {
	if stats.Blkio != nil {
		s.BlkioStats = types.BlkioStats{
			IoServiceBytesRecursive: copyBlkioEntry(stats.Blkio.IoServiceBytesRecursive),
			IoServicedRecursive:     copyBlkioEntry(stats.Blkio.IoServicedRecursive),
			IoQueuedRecursive:       copyBlkioEntry(stats.Blkio.IoQueuedRecursive),
			IoServiceTimeRecursive:  copyBlkioEntry(stats.Blkio.IoServiceTimeRecursive),
			IoWaitTimeRecursive:     copyBlkioEntry(stats.Blkio.IoWaitTimeRecursive),
			IoMergedRecursive:       copyBlkioEntry(stats.Blkio.IoMergedRecursive),
			IoTimeRecursive:         copyBlkioEntry(stats.Blkio.IoTimeRecursive),
			SectorsRecursive:        copyBlkioEntry(stats.Blkio.SectorsRecursive),
		}
	}
	if stats.CPU != nil {
		s.CPUStats = types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage:        stats.CPU.Usage.Total,
				PercpuUsage:       stats.CPU.Usage.PerCPU,
				UsageInKernelmode: stats.CPU.Usage.Kernel,
				UsageInUsermode:   stats.CPU.Usage.User,
			},
			ThrottlingData: types.ThrottlingData{
				Periods:          stats.CPU.Throttling.Periods,
				ThrottledPeriods: stats.CPU.Throttling.ThrottledPeriods,
				ThrottledTime:    stats.CPU.Throttling.ThrottledTime,
			},
		}
	}

	if stats.Memory != nil {
		raw := map[string]uint64{
			"cache":                     stats.Memory.Cache,
			"rss":                       stats.Memory.RSS,
			"rss_huge":                  stats.Memory.RSSHuge,
			"mapped_file":               stats.Memory.MappedFile,
			"dirty":                     stats.Memory.Dirty,
			"writeback":                 stats.Memory.Writeback,
			"pgpgin":                    stats.Memory.PgPgIn,
			"pgpgout":                   stats.Memory.PgPgOut,
			"pgfault":                   stats.Memory.PgFault,
			"pgmajfault":                stats.Memory.PgMajFault,
			"inactive_anon":             stats.Memory.InactiveAnon,
			"active_anon":               stats.Memory.ActiveAnon,
			"inactive_file":             stats.Memory.InactiveFile,
			"active_file":               stats.Memory.ActiveFile,
			"unevictable":               stats.Memory.Unevictable,
			"hierarchical_memory_limit": stats.Memory.HierarchicalMemoryLimit,
			"hierarchical_memsw_limit":  stats.Memory.HierarchicalSwapLimit,
			"total_cache":               stats.Memory.TotalCache,
			"total_rss":                 stats.Memory.TotalRSS,
			"total_rss_huge":            stats.Memory.TotalRSSHuge,
			"total_mapped_file":         stats.Memory.TotalMappedFile,
			"total_dirty":               stats.Memory.TotalDirty,
			"total_writeback":           stats.Memory.TotalWriteback,
			"total_pgpgin":              stats.Memory.TotalPgPgIn,
			"total_pgpgout":             stats.Memory.TotalPgPgOut,
			"total_pgfault":             stats.Memory.TotalPgFault,
			"total_pgmajfault":          stats.Memory.TotalPgMajFault,
			"total_inactive_anon":       stats.Memory.TotalInactiveAnon,
			"total_active_anon":         stats.Memory.TotalActiveAnon,
			"total_inactive_file":       stats.Memory.TotalInactiveFile,
			"total_active_file":         stats.Memory.TotalActiveFile,
			"total_unevictable":         stats.Memory.TotalUnevictable,
		}
		if stats.Memory.Usage != nil {
			s.MemoryStats = types.MemoryStats{
				Stats:    raw,
				Usage:    stats.Memory.Usage.Usage,
				MaxUsage: stats.Memory.Usage.Max,
				Limit:    stats.Memory.Usage.Limit,
				Failcnt:  stats.Memory.Usage.Failcnt,
			}
		} else {
			s.MemoryStats = types.MemoryStats{
				Stats: raw,
			}
		}

		// if the container does not set memory limit, use the machineMemory
		if s.MemoryStats.Limit > daemon.machineMemory && daemon.machineMemory > 0 {
			s.MemoryStats.Limit = daemon.machineMemory
		}
	}

	if stats.Pids != nil {
		s.PidsStats = types.PidsStats{
			Current: stats.Pids.Current,
			Limit:   stats.Pids.Limit,
		}
	}

	return s, nil
}

func (daemon *Daemon) statsV2(s *types.StatsJSON, stats *statsV2.Metrics) (*types.StatsJSON, error) {
	if stats.Io != nil {
		var isbr []types.BlkioStatEntry
		for _, re := range stats.Io.Usage {
			isbr = append(isbr,
				types.BlkioStatEntry{
					Major: re.Major,
					Minor: re.Minor,
					Op:    "read",
					Value: re.Rbytes,
				},
				types.BlkioStatEntry{
					Major: re.Major,
					Minor: re.Minor,
					Op:    "write",
					Value: re.Wbytes,
				},
			)
		}
		s.BlkioStats = types.BlkioStats{
			IoServiceBytesRecursive: isbr,
			// Other fields are unsupported
		}
	}

	if stats.CPU != nil {
		s.CPUStats = types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage: stats.CPU.UsageUsec * 1000,
				// PercpuUsage is not supported
				UsageInKernelmode: stats.CPU.SystemUsec * 1000,
				UsageInUsermode:   stats.CPU.UserUsec * 1000,
			},
			ThrottlingData: types.ThrottlingData{
				Periods:          stats.CPU.NrPeriods,
				ThrottledPeriods: stats.CPU.NrThrottled,
				ThrottledTime:    stats.CPU.ThrottledUsec * 1000,
			},
		}
	}

	if stats.Memory != nil {
		s.MemoryStats = types.MemoryStats{
			// Stats is not compatible with v1
			Stats: map[string]uint64{
				"anon":                   stats.Memory.Anon,
				"file":                   stats.Memory.File,
				"kernel_stack":           stats.Memory.KernelStack,
				"slab":                   stats.Memory.Slab,
				"sock":                   stats.Memory.Sock,
				"shmem":                  stats.Memory.Shmem,
				"file_mapped":            stats.Memory.FileMapped,
				"file_dirty":             stats.Memory.FileDirty,
				"file_writeback":         stats.Memory.FileWriteback,
				"anon_thp":               stats.Memory.AnonThp,
				"inactive_anon":          stats.Memory.InactiveAnon,
				"active_anon":            stats.Memory.ActiveAnon,
				"inactive_file":          stats.Memory.InactiveFile,
				"active_file":            stats.Memory.ActiveFile,
				"unevictable":            stats.Memory.Unevictable,
				"slab_reclaimable":       stats.Memory.SlabReclaimable,
				"slab_unreclaimable":     stats.Memory.SlabUnreclaimable,
				"pgfault":                stats.Memory.Pgfault,
				"pgmajfault":             stats.Memory.Pgmajfault,
				"workingset_refault":     stats.Memory.WorkingsetRefault,
				"workingset_activate":    stats.Memory.WorkingsetActivate,
				"workingset_nodereclaim": stats.Memory.WorkingsetNodereclaim,
				"pgrefill":               stats.Memory.Pgrefill,
				"pgscan":                 stats.Memory.Pgscan,
				"pgsteal":                stats.Memory.Pgsteal,
				"pgactivate":             stats.Memory.Pgactivate,
				"pgdeactivate":           stats.Memory.Pgdeactivate,
				"pglazyfree":             stats.Memory.Pglazyfree,
				"pglazyfreed":            stats.Memory.Pglazyfreed,
				"thp_fault_alloc":        stats.Memory.ThpFaultAlloc,
				"thp_collapse_alloc":     stats.Memory.ThpCollapseAlloc,
			},
			Usage: stats.Memory.Usage,
			// MaxUsage is not supported
			Limit: stats.Memory.UsageLimit,
		}
		// if the container does not set memory limit, use the machineMemory
		if s.MemoryStats.Limit > daemon.machineMemory && daemon.machineMemory > 0 {
			s.MemoryStats.Limit = daemon.machineMemory
		}
		if stats.MemoryEvents != nil {
			// Failcnt is set to the "oom" field of the "memory.events" file.
			// See https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html
			s.MemoryStats.Failcnt = stats.MemoryEvents.Oom
		}
	}

	if stats.Pids != nil {
		s.PidsStats = types.PidsStats{
			Current: stats.Pids.Current,
			Limit:   stats.Pids.Limit,
		}
	}

	return s, nil
}

// Resolve Network SandboxID in case the container reuse another container's network stack
func (daemon *Daemon) getNetworkSandboxID(c *container.Container) (string, error) {
	curr := c
	for curr.HostConfig.NetworkMode.IsContainer() {
		containerID := curr.HostConfig.NetworkMode.ConnectedContainer()
		connected, err := daemon.GetContainer(containerID)
		if err != nil {
			return "", errors.Wrapf(err, "Could not get container for %s", containerID)
		}
		curr = connected
	}
	return curr.NetworkSettings.SandboxID, nil
}

func (daemon *Daemon) getNetworkStats(c *container.Container) (map[string]types.NetworkStats, error) {
	sandboxID, err := daemon.getNetworkSandboxID(c)
	if err != nil {
		return nil, err
	}

	sb, err := daemon.netController.SandboxByID(sandboxID)
	if err != nil {
		return nil, err
	}

	lnstats, err := sb.Statistics()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]types.NetworkStats)
	// Convert libnetwork nw stats into api stats
	for ifName, ifStats := range lnstats {
		stats[ifName] = types.NetworkStats{
			RxBytes:   ifStats.RxBytes,
			RxPackets: ifStats.RxPackets,
			RxErrors:  ifStats.RxErrors,
			RxDropped: ifStats.RxDropped,
			TxBytes:   ifStats.TxBytes,
			TxPackets: ifStats.TxPackets,
			TxErrors:  ifStats.TxErrors,
			TxDropped: ifStats.TxDropped,
		}
	}

	return stats, nil
}

const (
	// The value comes from `C.sysconf(C._SC_CLK_TCK)`, and
	// on Linux it's a constant which is safe to be hard coded,
	// so we can avoid using cgo here. For details, see:
	// https://github.com/containerd/cgroups/pull/12
	clockTicksPerSecond  = 100
	nanoSecondsPerSecond = 1e9
)

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds and number of online CPUs. An error is returned
// if the format of the underlying file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func getSystemCPUUsage() (cpuUsage uint64, cpuNum uint32, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 || line[:3] != "cpu" {
			break // Assume all cpu* records are at the front, like glibc https://github.com/bminor/glibc/blob/5d00c201b9a2da768a79ea8d5311f257871c0b43/sysdeps/unix/sysv/linux/getsysstats.c#L108-L135
		}
		if line[3] == ' ' {
			parts := strings.Fields(line)
			if len(parts) < 8 {
				return 0, 0, fmt.Errorf("invalid number of cpu fields")
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, 0, fmt.Errorf("Unable to convert value %s to int: %w", i, err)
				}
				totalClockTicks += v
			}
			cpuUsage = (totalClockTicks * nanoSecondsPerSecond) /
				clockTicksPerSecond
		}
		if '0' <= line[3] && line[3] <= '9' {
			cpuNum++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("error scanning '/proc/stat' file: %w", err)
	}
	return
}
