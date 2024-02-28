package resources

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/pkg/errors"
)

const (
	ioStatFile     = "io.stat"
	ioPressureFile = "io.pressure"
)

const (
	ioReadBytes    = "rbytes"
	ioWriteBytes   = "wbytes"
	ioDiscardBytes = "dbytes"
	ioReadIOs      = "rios"
	ioWriteIOs     = "wios"
	ioDiscardIOs   = "dios"
)

func getCgroupIOStat(cgroupPath string) (*resourcestypes.IOStat, error) {
	ioStatPath := filepath.Join(cgroupPath, ioStatFile)
	data, err := os.ReadFile(ioStatPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to read %s", ioStatPath)
	}

	ioStat := &resourcestypes.IOStat{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		for _, part := range parts[1:] {
			key, value := parseKeyValue(part)
			if key == "" {
				continue
			}

			switch key {
			case ioReadBytes:
				if ioStat.ReadBytes != nil {
					*ioStat.ReadBytes += value
				} else {
					ioStat.ReadBytes = uint64Ptr(value)
				}
			case ioWriteBytes:
				if ioStat.WriteBytes != nil {
					*ioStat.WriteBytes += value
				} else {
					ioStat.WriteBytes = uint64Ptr(value)
				}
			case ioDiscardBytes:
				if ioStat.DiscardBytes != nil {
					*ioStat.DiscardBytes += value
				} else {
					ioStat.DiscardBytes = uint64Ptr(value)
				}
			case ioReadIOs:
				if ioStat.ReadIOs != nil {
					*ioStat.ReadIOs += value
				} else {
					ioStat.ReadIOs = uint64Ptr(value)
				}
			case ioWriteIOs:
				if ioStat.WriteIOs != nil {
					*ioStat.WriteIOs += value
				} else {
					ioStat.WriteIOs = uint64Ptr(value)
				}
			case ioDiscardIOs:
				if ioStat.DiscardIOs != nil {
					*ioStat.DiscardIOs += value
				} else {
					ioStat.DiscardIOs = uint64Ptr(value)
				}
			}
		}
	}

	// Parse the pressure
	pressure, err := parsePressureFile(filepath.Join(cgroupPath, ioPressureFile))
	if err != nil {
		return nil, err
	}
	ioStat.Pressure = pressure

	return ioStat, nil
}

func parseKeyValue(kv string) (key string, value uint64) {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 {
		return "", 0
	}
	key = parts[0]
	value, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0
	}
	return key, value
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}
