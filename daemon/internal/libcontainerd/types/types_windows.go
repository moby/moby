package types

import (
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	wstats "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/stats"
)

type Summary options.ProcessDetails

// Stats contains statistics from HCS
type Stats struct {
	Read     time.Time
	HCSStats *hcsshim.Statistics
}

// InterfaceToStats returns a stats object from the platform-specific interface.
// The builtin HCS runtime emits *hcsshim.Statistics; the runhcs shim (containerd
// runtime) emits *wstats.Statistics, which is converted to the same shape.
func InterfaceToStats(read time.Time, v any) *Stats {
	switch stats := v.(type) {
	case *hcsshim.Statistics:
		return &Stats{
			HCSStats: stats,
			Read:     read,
		}
	case *wstats.Statistics:
		return &Stats{
			HCSStats: winStatsToHCSStats(stats),
			Read:     read,
		}
	default:
		return &Stats{
			Read: read,
		}
	}
}

// hcsRuntimeUnit converts the shim's nanosecond runtime values to the 100ns
// units used by hcsshim.Statistics.
const hcsRuntimeUnit = 100

// winStatsToHCSStats converts runhcs shim stats to the *hcsshim.Statistics shape
// consumed by stats_windows.go, returning nil for a non-Windows-container payload.
func winStatsToHCSStats(stats *wstats.Statistics) *hcsshim.Statistics {
	win := stats.GetWindows()
	if win == nil {
		return nil
	}

	hcss := &hcsshim.Statistics{}
	if ts := win.GetTimestamp(); ts != nil {
		hcss.Timestamp = ts.AsTime()
	}
	if ts := win.GetContainerStartTime(); ts != nil {
		hcss.ContainerStartTime = ts.AsTime()
	}
	hcss.Uptime100ns = win.GetUptimeNS() / hcsRuntimeUnit

	if proc := win.GetProcessor(); proc != nil {
		hcss.Processor = hcsshim.ProcessorStats{
			TotalRuntime100ns:  proc.GetTotalRuntimeNS() / hcsRuntimeUnit,
			RuntimeUser100ns:   proc.GetRuntimeUserNS() / hcsRuntimeUnit,
			RuntimeKernel100ns: proc.GetRuntimeKernelNS() / hcsRuntimeUnit,
		}
	}

	if mem := win.GetMemory(); mem != nil {
		hcss.Memory = hcsshim.MemoryStats{
			UsageCommitBytes:            mem.GetMemoryUsageCommitBytes(),
			UsageCommitPeakBytes:        mem.GetMemoryUsageCommitPeakBytes(),
			UsagePrivateWorkingSetBytes: mem.GetMemoryUsagePrivateWorkingSetBytes(),
		}
	}

	if storage := win.GetStorage(); storage != nil {
		hcss.Storage = hcsshim.StorageStats{
			ReadCountNormalized:  storage.GetReadCountNormalized(),
			ReadSizeBytes:        storage.GetReadSizeBytes(),
			WriteCountNormalized: storage.GetWriteCountNormalized(),
			WriteSizeBytes:       storage.GetWriteSizeBytes(),
		}
	}

	return hcss
}

// Resources defines updatable container resource values.
type Resources struct{}

// Checkpoint holds the details of a checkpoint (not supported in windows)
type Checkpoint struct {
	Name string
}

// Checkpoints contains the details of a checkpoint
type Checkpoints struct {
	Checkpoints []*Checkpoint
}
