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
	memoryStatFile        = "memory.stat"
	memoryPressureFile    = "memory.pressure"
	memoryPeakFile        = "memory.peak"
	memorySwapCurrentFile = "memory.swap.current"
	memoryEventsFile      = "memory.events"
)

const (
	memoryAnon          = "anon"
	memoryFile          = "file"
	memoryKernelStack   = "kernel_stack"
	memoryPageTables    = "pagetables"
	memorySock          = "sock"
	memoryShmem         = "shmem"
	memoryFileMapped    = "file_mapped"
	memoryFileDirty     = "file_dirty"
	memoryFileWriteback = "file_writeback"
	memorySlab          = "slab"
	memoryPgscan        = "pgscan"
	memoryPgsteal       = "pgsteal"
	memoryPgfault       = "pgfault"
	memoryPgmajfault    = "pgmajfault"

	memoryLow     = "low"
	memoryHigh    = "high"
	memoryMax     = "max"
	memoryOom     = "oom"
	memoryOomKill = "oom_kill"
)

func getCgroupMemoryStat(path string) (*resourcestypes.MemoryStat, error) {
	memoryStat := &resourcestypes.MemoryStat{}

	// Parse memory.stat
	err := parseKeyValueFile(filepath.Join(path, memoryStatFile), func(key string, value uint64) {
		switch key {
		case memoryAnon:
			memoryStat.Anon = &value
		case memoryFile:
			memoryStat.File = &value
		case memoryKernelStack:
			memoryStat.KernelStack = &value
		case memoryPageTables:
			memoryStat.PageTables = &value
		case memorySock:
			memoryStat.Sock = &value
		case memoryShmem:
			memoryStat.Shmem = &value
		case memoryFileMapped:
			memoryStat.FileMapped = &value
		case memoryFileDirty:
			memoryStat.FileDirty = &value
		case memoryFileWriteback:
			memoryStat.FileWriteback = &value
		case memorySlab:
			memoryStat.Slab = &value
		case memoryPgscan:
			memoryStat.Pgscan = &value
		case memoryPgsteal:
			memoryStat.Pgsteal = &value
		case memoryPgfault:
			memoryStat.Pgfault = &value
		case memoryPgmajfault:
			memoryStat.Pgmajfault = &value
		}
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	pressure, err := parsePressureFile(filepath.Join(path, memoryPressureFile))
	if err != nil {
		return nil, err
	}
	if pressure != nil {
		memoryStat.Pressure = pressure
	}

	err = parseKeyValueFile(filepath.Join(path, memoryEventsFile), func(key string, value uint64) {
		switch key {
		case memoryLow:
			memoryStat.LowEvents = value
		case memoryHigh:
			memoryStat.HighEvents = value
		case memoryMax:
			memoryStat.MaxEvents = value
		case memoryOom:
			memoryStat.OomEvents = value
		case memoryOomKill:
			memoryStat.OomKillEvents = value
		}
	})

	if err != nil {
		return nil, err
	}

	peak, err := parseSingleValueFile(filepath.Join(path, memoryPeakFile))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		memoryStat.Peak = &peak
	}

	swap, err := parseSingleValueFile(filepath.Join(path, memorySwapCurrentFile))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		memoryStat.SwapBytes = &swap
	}

	return memoryStat, nil
}

func parseKeyValueFile(filePath string, callback func(key string, value uint64)) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read %s", filePath)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		fields := strings.Fields(line)
		key := fields[0]
		valueStr := fields[1]
		value, err := strconv.ParseUint(valueStr, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse value for %s", key)
		}

		callback(key, value)
	}

	return nil
}
