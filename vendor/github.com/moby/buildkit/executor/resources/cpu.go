package resources

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/pkg/errors"
)

const (
	cpuUsageUsec     = "usage_usec"
	cpuUserUsec      = "user_usec"
	cpuSystemUsec    = "system_usec"
	cpuNrPeriods     = "nr_periods"
	cpuNrThrottled   = "nr_throttled"
	cpuThrottledUsec = "throttled_usec"
)

func getCgroupCPUStat(cgroupPath string) (*resourcestypes.CPUStat, error) {
	cpuStat := &resourcestypes.CPUStat{}

	// Read cpu.stat file
	cpuStatFile, err := os.Open(filepath.Join(cgroupPath, "cpu.stat"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer cpuStatFile.Close()

	scanner := bufio.NewScanner(cpuStatFile)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 2 {
			continue
		}

		key := fields[0]
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}

		switch key {
		case cpuUsageUsec:
			cpuStat.UsageNanos = uint64Ptr(value * 1000)
		case cpuUserUsec:
			cpuStat.UserNanos = uint64Ptr(value * 1000)
		case cpuSystemUsec:
			cpuStat.SystemNanos = uint64Ptr(value * 1000)
		case cpuNrPeriods:
			cpuStat.NrPeriods = new(uint32)
			*cpuStat.NrPeriods = uint32(value)
		case cpuNrThrottled:
			cpuStat.NrThrottled = new(uint32)
			*cpuStat.NrThrottled = uint32(value)
		case cpuThrottledUsec:
			cpuStat.ThrottledNanos = uint64Ptr(value * 1000)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Read cpu.pressure file
	pressure, err := parsePressureFile(filepath.Join(cgroupPath, "cpu.pressure"))
	if err == nil {
		cpuStat.Pressure = pressure
	}

	return cpuStat, nil
}
func parsePressureFile(filename string) (*resourcestypes.Pressure, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTSUP) { // pressure file requires CONFIG_PSI
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	pressure := &resourcestypes.Pressure{}
	for _, line := range lines {
		// Skip empty lines
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		fields := strings.Fields(line)
		prefix := fields[0]
		pressureValues := &resourcestypes.PressureValues{}

		for i := 1; i < len(fields); i++ {
			keyValue := strings.Split(fields[i], "=")
			key := keyValue[0]
			valueStr := keyValue[1]

			if key == "total" {
				totalValue, err := strconv.ParseUint(valueStr, 10, 64)
				if err != nil {
					return nil, err
				}
				pressureValues.Total = &totalValue
			} else {
				value, err := strconv.ParseFloat(valueStr, 64)
				if err != nil {
					return nil, err
				}

				switch key {
				case "avg10":
					pressureValues.Avg10 = &value
				case "avg60":
					pressureValues.Avg60 = &value
				case "avg300":
					pressureValues.Avg300 = &value
				}
			}
		}

		switch prefix {
		case "some":
			pressure.Some = pressureValues
		case "full":
			pressure.Full = pressureValues
		}
	}

	return pressure, nil
}
