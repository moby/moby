package types

import (
	"testing"
	"time"

	"github.com/Microsoft/hcsshim"
	wstats "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/stats"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestInterfaceToStatsHCS(t *testing.T) {
	read := time.Now()
	in := &hcsshim.Statistics{
		Processor: hcsshim.ProcessorStats{TotalRuntime100ns: 1234},
	}

	got := InterfaceToStats(read, in)
	if got.HCSStats != in {
		t.Fatalf("expected the original *hcsshim.Statistics to be passed through unchanged")
	}
	if !got.Read.Equal(read) {
		t.Fatalf("Read = %v, want %v", got.Read, read)
	}
}

func TestInterfaceToStatsRunhcs(t *testing.T) {
	read := time.Now()
	start := time.Unix(1000, 0).UTC()
	ts := time.Unix(2000, 0).UTC()

	in := &wstats.Statistics{
		Container: &wstats.Statistics_Windows{
			Windows: &wstats.WindowsContainerStatistics{
				Timestamp:          timestamppb.New(ts),
				ContainerStartTime: timestamppb.New(start),
				UptimeNS:           500,
				Processor: &wstats.WindowsContainerProcessorStatistics{
					TotalRuntimeNS:  100000,
					RuntimeUserNS:   60000,
					RuntimeKernelNS: 40000,
				},
				Memory: &wstats.WindowsContainerMemoryStatistics{
					MemoryUsageCommitBytes:            111,
					MemoryUsageCommitPeakBytes:        222,
					MemoryUsagePrivateWorkingSetBytes: 333,
				},
				Storage: &wstats.WindowsContainerStorageStatistics{
					ReadCountNormalized:  1,
					ReadSizeBytes:        2,
					WriteCountNormalized: 3,
					WriteSizeBytes:       4,
				},
			},
		},
	}

	got := InterfaceToStats(read, in)
	if !got.Read.Equal(read) {
		t.Fatalf("Read = %v, want %v", got.Read, read)
	}
	if got.HCSStats == nil {
		t.Fatal("HCSStats is nil, want populated stats")
	}
	hcss := got.HCSStats

	if !hcss.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", hcss.Timestamp, ts)
	}
	if !hcss.ContainerStartTime.Equal(start) {
		t.Errorf("ContainerStartTime = %v, want %v", hcss.ContainerStartTime, start)
	}
	// Runtime fields are reported in nanoseconds and converted to 100ns units.
	if hcss.Uptime100ns != 5 {
		t.Errorf("Uptime100ns = %d, want 5", hcss.Uptime100ns)
	}
	if hcss.Processor.TotalRuntime100ns != 1000 {
		t.Errorf("TotalRuntime100ns = %d, want 1000", hcss.Processor.TotalRuntime100ns)
	}
	if hcss.Processor.RuntimeUser100ns != 600 {
		t.Errorf("RuntimeUser100ns = %d, want 600", hcss.Processor.RuntimeUser100ns)
	}
	if hcss.Processor.RuntimeKernel100ns != 400 {
		t.Errorf("RuntimeKernel100ns = %d, want 400", hcss.Processor.RuntimeKernel100ns)
	}
	if hcss.Memory.UsageCommitBytes != 111 {
		t.Errorf("UsageCommitBytes = %d, want 111", hcss.Memory.UsageCommitBytes)
	}
	if hcss.Memory.UsageCommitPeakBytes != 222 {
		t.Errorf("UsageCommitPeakBytes = %d, want 222", hcss.Memory.UsageCommitPeakBytes)
	}
	if hcss.Memory.UsagePrivateWorkingSetBytes != 333 {
		t.Errorf("UsagePrivateWorkingSetBytes = %d, want 333", hcss.Memory.UsagePrivateWorkingSetBytes)
	}
	if hcss.Storage.ReadCountNormalized != 1 || hcss.Storage.ReadSizeBytes != 2 ||
		hcss.Storage.WriteCountNormalized != 3 || hcss.Storage.WriteSizeBytes != 4 {
		t.Errorf("Storage = %+v, want {1 2 3 4}", hcss.Storage)
	}
}

func TestInterfaceToStatsRunhcsNoWindows(t *testing.T) {
	// A non-Windows-container payload yields nil HCSStats rather than panicking.
	got := InterfaceToStats(time.Now(), &wstats.Statistics{})
	if got.HCSStats != nil {
		t.Fatalf("HCSStats = %+v, want nil", got.HCSStats)
	}
}
