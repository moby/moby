//go:build !windows
// +build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/container"
	"github.com/pkg/errors"
)

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
