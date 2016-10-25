// +build !windows,!solaris

package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	sysinfo "github.com/docker/docker/pkg/system"
	"github.com/opencontainers/runc/libcontainer/system"
)

// platformNewStatsCollector performs platform specific initialisation of the
// statsCollector structure.
func platformNewStatsCollector(s *statsCollector) {
	s.clockTicksPerSecond = uint64(system.GetClockTicks())
	meminfo, err := sysinfo.ReadMemInfo()
	if err == nil && meminfo.MemTotal > 0 {
		s.machineMemory = uint64(meminfo.MemTotal)
	}
}

const nanoSecondsPerSecond = 1e9

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func (s *statsCollector) getSystemCPUUsage() (uint64, error) {
	var line string
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer func() {
		s.bufReader.Reset(nil)
		f.Close()
	}()
	s.bufReader.Reset(f)
	err = nil
	for err == nil {
		line, err = s.bufReader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				totalClockTicks += v
			}
			return (totalClockTicks * nanoSecondsPerSecond) /
				s.clockTicksPerSecond, nil
		}
	}
	return 0, fmt.Errorf("invalid stat format. Error trying to parse the '/proc/stat' file")
}
