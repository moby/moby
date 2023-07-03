/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cgroup2

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/containerd/cgroups/v3/cgroup2/stats"

	"github.com/godbus/dbus/v5"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	cgroupProcs    = "cgroup.procs"
	cgroupThreads  = "cgroup.threads"
	defaultDirPerm = 0o755
)

// defaultFilePerm is a var so that the test framework can change the filemode
// of all files created when the tests are running.  The difference between the
// tests and real world use is that files like "cgroup.procs" will exist when writing
// to a read cgroup filesystem and do not exist prior when running in the tests.
// this is set to a non 0 value in the test code
var defaultFilePerm = os.FileMode(0)

// remove will remove a cgroup path handling EAGAIN and EBUSY errors and
// retrying the remove after a exp timeout
func remove(path string) error {
	var err error
	delay := 10 * time.Millisecond
	for i := 0; i < 5; i++ {
		if i != 0 {
			time.Sleep(delay)
			delay *= 2
		}
		if err = os.RemoveAll(path); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cgroups: unable to remove path %q: %w", path, err)
}

// parseCgroupProcsFile parses /sys/fs/cgroup/$GROUPPATH/cgroup.procs
func parseCgroupProcsFile(path string) ([]uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var (
		out []uint64
		s   = bufio.NewScanner(f)
	)
	for s.Scan() {
		if t := s.Text(); t != "" {
			pid, err := strconv.ParseUint(t, 10, 0)
			if err != nil {
				return nil, err
			}
			out = append(out, pid)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseKV(raw string) (string, uint64, error) {
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return "", 0, ErrInvalidFormat
	}
	v, err := parseUint(parts[1], 10, 64)
	return parts[0], v, err
}

func parseUint(s string, base, bitSize int) (uint64, error) {
	v, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		// 1. Handle negative values greater than MinInt64 (and)
		// 2. Handle negative values lesser than MinInt64
		if intErr == nil && intValue < 0 {
			return 0, nil
		} else if intErr != nil &&
			intErr.(*strconv.NumError).Err == strconv.ErrRange &&
			intValue < 0 {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

// parseCgroupFile parses /proc/PID/cgroup file and return string
func parseCgroupFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return parseCgroupFromReader(f)
}

func parseCgroupFromReader(r io.Reader) (string, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		var (
			text  = s.Text()
			parts = strings.SplitN(text, ":", 3)
		)
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid cgroup entry: %q", text)
		}
		// text is like "0::/user.slice/user-1001.slice/session-1.scope"
		if parts[0] == "0" && parts[1] == "" {
			return parts[2], nil
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("cgroup path not found")
}

// ToResources converts the oci LinuxResources struct into a
// v2 Resources type for use with this package.
//
// converting cgroups configuration from v1 to v2
// ref: https://github.com/containers/crun/blob/master/crun.1.md#cgroup-v2
func ToResources(spec *specs.LinuxResources) *Resources {
	var resources Resources
	if cpu := spec.CPU; cpu != nil {
		resources.CPU = &CPU{
			Cpus: cpu.Cpus,
			Mems: cpu.Mems,
		}
		if shares := cpu.Shares; shares != nil {
			convertedWeight := 1 + ((*shares-2)*9999)/262142
			resources.CPU.Weight = &convertedWeight
		}
		if period := cpu.Period; period != nil {
			resources.CPU.Max = NewCPUMax(cpu.Quota, period)
		}
	}
	if mem := spec.Memory; mem != nil {
		resources.Memory = &Memory{}
		if swap := mem.Swap; swap != nil {
			resources.Memory.Swap = swap
		}
		if l := mem.Limit; l != nil {
			resources.Memory.Max = l
		}
		if l := mem.Reservation; l != nil {
			resources.Memory.Low = l
		}
	}
	if hugetlbs := spec.HugepageLimits; hugetlbs != nil {
		hugeTlbUsage := HugeTlb{}
		for _, hugetlb := range hugetlbs {
			hugeTlbUsage = append(hugeTlbUsage, HugeTlbEntry{
				HugePageSize: hugetlb.Pagesize,
				Limit:        hugetlb.Limit,
			})
		}
		resources.HugeTlb = &hugeTlbUsage
	}
	if pids := spec.Pids; pids != nil {
		resources.Pids = &Pids{
			Max: pids.Limit,
		}
	}
	if i := spec.BlockIO; i != nil {
		resources.IO = &IO{}
		if i.Weight != nil {
			resources.IO.BFQ.Weight = 1 + (*i.Weight-10)*9999/990
		}
		for t, devices := range map[IOType][]specs.LinuxThrottleDevice{
			ReadBPS:   i.ThrottleReadBpsDevice,
			WriteBPS:  i.ThrottleWriteBpsDevice,
			ReadIOPS:  i.ThrottleReadIOPSDevice,
			WriteIOPS: i.ThrottleWriteIOPSDevice,
		} {
			for _, d := range devices {
				resources.IO.Max = append(resources.IO.Max, Entry{
					Type:  t,
					Major: d.Major,
					Minor: d.Minor,
					Rate:  d.Rate,
				})
			}
		}
	}
	if i := spec.Rdma; i != nil {
		resources.RDMA = &RDMA{}
		for device, value := range spec.Rdma {
			if device != "" && (value.HcaHandles != nil && value.HcaObjects != nil) {
				resources.RDMA.Limit = append(resources.RDMA.Limit, RDMAEntry{
					Device:     device,
					HcaHandles: *value.HcaHandles,
					HcaObjects: *value.HcaObjects,
				})
			}
		}
	}

	return &resources
}

// Gets uint64 parsed content of single value cgroup stat file
func getStatFileContentUint64(filePath string) uint64 {
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()

	// We expect an unsigned 64 bit integer, or a "max" string
	// in some cases.
	buf := make([]byte, 32)
	n, err := f.Read(buf)
	if err != nil {
		return 0
	}

	trimmed := strings.TrimSpace(string(buf[:n]))
	if trimmed == "max" {
		return math.MaxUint64
	}

	res, err := parseUint(trimmed, 10, 64)
	if err != nil {
		logrus.Errorf("unable to parse %q as a uint from Cgroup file %q", trimmed, filePath)
		return res
	}

	return res
}

func readIoStats(path string) []*stats.IOEntry {
	// more details on the io.stat file format: https://www.kernel.org/doc/Documentation/cgroup-v2.txt
	var usage []*stats.IOEntry
	fpath := filepath.Join(path, "io.stat")
	currentData, err := os.ReadFile(fpath)
	if err != nil {
		return usage
	}
	entries := strings.Split(string(currentData), "\n")

	for _, entry := range entries {
		parts := strings.Split(entry, " ")
		if len(parts) < 2 {
			continue
		}
		majmin := strings.Split(parts[0], ":")
		if len(majmin) != 2 {
			continue
		}
		major, err := strconv.ParseUint(majmin[0], 10, 0)
		if err != nil {
			return usage
		}
		minor, err := strconv.ParseUint(majmin[1], 10, 0)
		if err != nil {
			return usage
		}
		parts = parts[1:]
		ioEntry := stats.IOEntry{
			Major: major,
			Minor: minor,
		}
		for _, s := range parts {
			keyPairValue := strings.Split(s, "=")
			if len(keyPairValue) != 2 {
				continue
			}
			v, err := strconv.ParseUint(keyPairValue[1], 10, 0)
			if err != nil {
				continue
			}
			switch keyPairValue[0] {
			case "rbytes":
				ioEntry.Rbytes = v
			case "wbytes":
				ioEntry.Wbytes = v
			case "rios":
				ioEntry.Rios = v
			case "wios":
				ioEntry.Wios = v
			}
		}
		usage = append(usage, &ioEntry)
	}
	return usage
}

func rdmaStats(filepath string) []*stats.RdmaEntry {
	currentData, err := os.ReadFile(filepath)
	if err != nil {
		return []*stats.RdmaEntry{}
	}
	return toRdmaEntry(strings.Split(string(currentData), "\n"))
}

func parseRdmaKV(raw string, entry *stats.RdmaEntry) {
	var value uint64
	var err error

	parts := strings.Split(raw, "=")
	switch len(parts) {
	case 2:
		if parts[1] == "max" {
			value = math.MaxUint32
		} else {
			value, err = parseUint(parts[1], 10, 32)
			if err != nil {
				return
			}
		}
		if parts[0] == "hca_handle" {
			entry.HcaHandles = uint32(value)
		} else if parts[0] == "hca_object" {
			entry.HcaObjects = uint32(value)
		}
	}
}

func toRdmaEntry(strEntries []string) []*stats.RdmaEntry {
	var rdmaEntries []*stats.RdmaEntry
	for i := range strEntries {
		parts := strings.Fields(strEntries[i])
		switch len(parts) {
		case 3:
			entry := new(stats.RdmaEntry)
			entry.Device = parts[0]
			parseRdmaKV(parts[1], entry)
			parseRdmaKV(parts[2], entry)

			rdmaEntries = append(rdmaEntries, entry)
		default:
			continue
		}
	}
	return rdmaEntries
}

// isUnitExists returns true if the error is that a systemd unit already exists.
func isUnitExists(err error) bool {
	if err != nil {
		if dbusError, ok := err.(dbus.Error); ok {
			return strings.Contains(dbusError.Name, "org.freedesktop.systemd1.UnitExists")
		}
	}
	return false
}

func systemdUnitFromPath(path string) string {
	_, unit := filepath.Split(path)
	return unit
}

func readHugeTlbStats(path string) []*stats.HugeTlbStat {
	hpSizes := hugePageSizes()
	usage := make([]*stats.HugeTlbStat, len(hpSizes))
	for idx, pagesize := range hpSizes {
		usage[idx] = &stats.HugeTlbStat{
			Max:      getStatFileContentUint64(filepath.Join(path, "hugetlb."+pagesize+".max")),
			Current:  getStatFileContentUint64(filepath.Join(path, "hugetlb."+pagesize+".current")),
			Pagesize: pagesize,
		}
	}
	return usage
}

var (
	hPageSizes  []string
	initHPSOnce sync.Once
)

// The following idea and implementation is taken pretty much line for line from
// runc. Because the hugetlb files are well known, and the only variable thrown in
// the mix is what huge page sizes you have on your host, this lends itself well
// to doing the work to find the files present once, and then re-using this. This
// saves a os.Readdirnames(0) call to search for hugeltb files on every `manager.Stat`
// call.
// https://github.com/opencontainers/runc/blob/3a2c0c2565644d8a7e0f1dd594a060b21fa96cf1/libcontainer/cgroups/utils.go#L301
func hugePageSizes() []string {
	initHPSOnce.Do(func() {
		dir, err := os.OpenFile("/sys/kernel/mm/hugepages", unix.O_DIRECTORY|unix.O_RDONLY, 0)
		if err != nil {
			return
		}
		files, err := dir.Readdirnames(0)
		dir.Close()
		if err != nil {
			return
		}

		hPageSizes, err = getHugePageSizeFromFilenames(files)
		if err != nil {
			logrus.Warnf("hugePageSizes: %s", err)
		}
	})

	return hPageSizes
}

func getHugePageSizeFromFilenames(fileNames []string) ([]string, error) {
	pageSizes := make([]string, 0, len(fileNames))
	var warn error

	for _, file := range fileNames {
		// example: hugepages-1048576kB
		val := strings.TrimPrefix(file, "hugepages-")
		if len(val) == len(file) {
			// Unexpected file name: no prefix found, ignore it.
			continue
		}
		// In all known versions of Linux up to 6.3 the suffix is always
		// "kB". If we find something else, produce an error but keep going.
		eLen := len(val) - 2
		val = strings.TrimSuffix(val, "kB")
		if len(val) != eLen {
			// Highly unlikely.
			if warn == nil {
				warn = errors.New(file + `: invalid suffix (expected "kB")`)
			}
			continue
		}
		size, err := strconv.Atoi(val)
		if err != nil {
			// Highly unlikely.
			if warn == nil {
				warn = fmt.Errorf("%s: %w", file, err)
			}
			continue
		}
		// Model after https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/mm/hugetlb_cgroup.c?id=eff48ddeab782e35e58ccc8853f7386bbae9dec4#n574
		// but in our case the size is in KB already.
		if size >= (1 << 20) {
			val = strconv.Itoa(size>>20) + "GB"
		} else if size >= (1 << 10) {
			val = strconv.Itoa(size>>10) + "MB"
		} else {
			val += "KB"
		}
		pageSizes = append(pageSizes, val)
	}

	return pageSizes, warn
}

func getSubreaper() (int, error) {
	var i uintptr
	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&i)), 0, 0, 0); err != nil {
		return -1, err
	}
	return int(i), nil
}
