package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/go-units"
)

// SysInfo stores information about which features a kernel supports.
// TODO Windows: Factor out platform specific capabilities.
type SysInfo struct {
	// Whether the kernel supports AppArmor or not
	AppArmor bool
	// Whether the kernel supports Seccomp or not
	Seccomp bool

	cgroupMemInfo
	cgroupHugetlbInfo
	cgroupCPUInfo
	cgroupBlkioInfo
	cgroupCpusetInfo
	cgroupPids

	// Whether IPv4 forwarding is supported or not, if this was disabled, networking will not work
	IPv4ForwardingDisabled bool

	// Whether bridge-nf-call-iptables is supported or not
	BridgeNFCallIPTablesDisabled bool

	// Whether bridge-nf-call-ip6tables is supported or not
	BridgeNFCallIP6TablesDisabled bool

	// Whether the cgroup has the mountpoint of "devices" or not
	CgroupDevicesEnabled bool
}

type cgroupMemInfo struct {
	// Whether memory limit is supported or not
	MemoryLimit bool

	// Whether swap limit is supported or not
	SwapLimit bool

	// Whether soft limit is supported or not
	MemoryReservation bool

	// Whether OOM killer disable is supported or not
	OomKillDisable bool

	// Whether memory swappiness is supported or not
	MemorySwappiness bool

	// Whether kernel memory limit is supported or not
	KernelMemory bool

	// Whether kernel memory TCP limit is supported or not
	KernelMemoryTCP bool
}

type cgroupHugetlbInfo struct {
	// Whether hugetlb limit is supported or not
	HugetlbLimit bool
}

type cgroupCPUInfo struct {
	// Whether CPU shares is supported or not
	CPUShares bool

	// Whether CPU CFS(Completely Fair Scheduler) period is supported or not
	CPUCfsPeriod bool

	// Whether CPU CFS(Completely Fair Scheduler) quota is supported or not
	CPUCfsQuota bool

	// Whether CPU real-time period is supported or not
	CPURealtimePeriod bool

	// Whether CPU real-time runtime is supported or not
	CPURealtimeRuntime bool
}

type cgroupBlkioInfo struct {
	// Whether Block IO weight is supported or not
	BlkioWeight bool

	// Whether Block IO weight_device is supported or not
	BlkioWeightDevice bool

	// Whether Block IO read limit in bytes per second is supported or not
	BlkioReadBpsDevice bool

	// Whether Block IO write limit in bytes per second is supported or not
	BlkioWriteBpsDevice bool

	// Whether Block IO read limit in IO per second is supported or not
	BlkioReadIOpsDevice bool

	// Whether Block IO write limit in IO per second is supported or not
	BlkioWriteIOpsDevice bool
}

type cgroupCpusetInfo struct {
	// Whether Cpuset is supported or not
	Cpuset bool

	// Available Cpuset's cpus
	Cpus string

	// Available Cpuset's memory nodes
	Mems string
}

type cgroupPids struct {
	// Whether Pids Limit is supported or not
	PidsLimit bool
}

// ValidateHugetlb check whether hugetlb pagesize and limit legal
func (c cgroupHugetlbInfo) ValidateHugetlb(pageSize string, limit uint64) (string, []string, error) {
	var (
		w   []string
		err error
	)
	if pageSize != "" {
		if err = isHugepageSizeValid(pageSize); err != nil {
			return "", w, err
		}
	} else {
		pageSize, err = c.GetDefaultHugepageSize()
		if err != nil {
			return "", w, fmt.Errorf("Failed to get system hugepage size")
		}
	}

	warning, err := isHugeLimitValid(pageSize, limit)
	w = append(w, warning...)
	if err != nil {
		return "", w, err
	}

	return pageSize, w, nil
}

// isHugeLimitValid check whether input hugetlb limit legal
// it will check whether the limit size is times of size
func isHugeLimitValid(size string, limit uint64) ([]string, error) {
	var w []string
	sizeInt, err := units.RAMInBytes(size)
	if err != nil || sizeInt < 0 {
		return w, fmt.Errorf("Invalid hugepage size:%s -- %s", size, err)
	}
	sizeUint := uint64(sizeInt)

	if limit%sizeUint != 0 {
		w = append(w, "Invalid hugetlb limit: should be multiple of huge page size")
	}

	return w, nil
}

// isHugepageSizeValid check whether input size legal
// it will compare size with all system supported hugepage size
func isHugepageSizeValid(size string) error {
	hps, err := getHugepageSizes()
	if err != nil {
		return err
	}

	for _, hp := range hps {
		if size == hp {
			return nil
		}
	}
	return fmt.Errorf("Invalid hugepage size:%s, shoud be one of %v", size, hps)
}

func humanSize(i int64) string {
	// hugetlb may not surpass GB
	uf := []string{"B", "KB", "MB", "GB"}
	ui := 0
	for {
		if i < 1024 || ui >= 3 {
			break
		}
		i = i / 1024
		ui = ui + 1
	}

	return fmt.Sprintf("%d%s", i, uf[ui])
}

func getHugepageSizes() ([]string, error) {
	var hps []string
	hpm := make(map[string]string)
	if err := filepath.Walk("/sys/fs/cgroup/hugetlb", func(p string, i os.FileInfo, err error) error {
		if strings.HasSuffix(i.Name(), "limit_in_bytes") {
			sres := strings.SplitN(i.Name(), ".", 3)
			if len(sres) != 3 {
				// just ignore this error and see if next file is as expected
				return nil
			}
			hpm[sres[1]] = sres[1]
		}
		return nil
	}); err != nil {
		return nil, err
	}

	for _, v := range hpm {
		hps = append(hps, v)
	}
	return hps, nil
}

// GetDefaultHugepageSize returns system default hugepage size
func (c cgroupHugetlbInfo) GetDefaultHugepageSize() (string, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return "", fmt.Errorf("Failed to get hugepage size, cannot open /proc/meminfo")
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.Contains(s.Text(), "Hugepagesize") {
			sres := strings.SplitN(s.Text(), ":", 2)
			if len(sres) != 2 {
				return "", fmt.Errorf("Failed to get hugepage size, wired /proc/meminfo format")
			}

			size := strings.Replace(sres[1], " ", "", -1)
			// transform 2048k to 2M
			sizeInt, _ := units.RAMInBytes(size)
			return humanSize(sizeInt), nil
		}
	}
	return "", fmt.Errorf("Failed to get hugepage size")
}

// IsCpusetCpusAvailable returns `true` if the provided string set is contained
// in cgroup's cpuset.cpus set, `false` otherwise.
// If error is not nil a parsing error occurred.
func (c cgroupCpusetInfo) IsCpusetCpusAvailable(provided string) (bool, error) {
	return isCpusetListAvailable(provided, c.Cpus)
}

// IsCpusetMemsAvailable returns `true` if the provided string set is contained
// in cgroup's cpuset.mems set, `false` otherwise.
// If error is not nil a parsing error occurred.
func (c cgroupCpusetInfo) IsCpusetMemsAvailable(provided string) (bool, error) {
	return isCpusetListAvailable(provided, c.Mems)
}

func isCpusetListAvailable(provided, available string) (bool, error) {
	parsedAvailable, err := parsers.ParseUintList(available)
	if err != nil {
		return false, err
	}
	// 8192 is the normal maximum number of CPUs in Linux, so accept numbers up to this
	// or more if we actually have more CPUs.
	max := 8192
	for m := range parsedAvailable {
		if m > max {
			max = m
		}
	}
	parsedProvided, err := parsers.ParseUintListMaximum(provided, max)
	if err != nil {
		return false, err
	}
	for k := range parsedProvided {
		if !parsedAvailable[k] {
			return false, nil
		}
	}
	return true, nil
}

// Returns bit count of 1, used by NumCPU
func popcnt(x uint64) (n byte) {
	x -= (x >> 1) & 0x5555555555555555
	x = (x>>2)&0x3333333333333333 + x&0x3333333333333333
	x += x >> 4
	x &= 0x0f0f0f0f0f0f0f0f
	x *= 0x0101010101010101
	return byte(x >> 56)
}
