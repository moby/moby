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

package cgroups

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewMemory(root string) *memoryController {
	return &memoryController{
		root: filepath.Join(root, string(Memory)),
	}
}

type memoryController struct {
	root string
}

func (m *memoryController) Name() Name {
	return Memory
}

func (m *memoryController) Path(path string) string {
	return filepath.Join(m.root, path)
}

func (m *memoryController) Create(path string, resources *specs.LinuxResources) error {
	if err := os.MkdirAll(m.Path(path), defaultDirPerm); err != nil {
		return err
	}
	if resources.Memory == nil {
		return nil
	}
	if resources.Memory.Kernel != nil {
		// Check if kernel memory is enabled
		// We have to limit the kernel memory here as it won't be accounted at all
		// until a limit is set on the cgroup and limit cannot be set once the
		// cgroup has children, or if there are already tasks in the cgroup.
		for _, i := range []int64{1, -1} {
			if err := ioutil.WriteFile(
				filepath.Join(m.Path(path), "memory.kmem.limit_in_bytes"),
				[]byte(strconv.FormatInt(i, 10)),
				defaultFilePerm,
			); err != nil {
				return checkEBUSY(err)
			}
		}
	}
	return m.set(path, getMemorySettings(resources))
}

func (m *memoryController) Update(path string, resources *specs.LinuxResources) error {
	if resources.Memory == nil {
		return nil
	}
	g := func(v *int64) bool {
		return v != nil && *v > 0
	}
	settings := getMemorySettings(resources)
	if g(resources.Memory.Limit) && g(resources.Memory.Swap) {
		// if the updated swap value is larger than the current memory limit set the swap changes first
		// then set the memory limit as swap must always be larger than the current limit
		current, err := readUint(filepath.Join(m.Path(path), "memory.limit_in_bytes"))
		if err != nil {
			return err
		}
		if current < uint64(*resources.Memory.Swap) {
			settings[0], settings[1] = settings[1], settings[0]
		}
	}
	return m.set(path, settings)
}

func (m *memoryController) Stat(path string, stats *Metrics) error {
	f, err := os.Open(filepath.Join(m.Path(path), "memory.stat"))
	if err != nil {
		return err
	}
	defer f.Close()
	stats.Memory = &MemoryStat{
		Usage:     &MemoryEntry{},
		Swap:      &MemoryEntry{},
		Kernel:    &MemoryEntry{},
		KernelTCP: &MemoryEntry{},
	}
	if err := m.parseStats(f, stats.Memory); err != nil {
		return err
	}
	for _, t := range []struct {
		module string
		entry  *MemoryEntry
	}{
		{
			module: "",
			entry:  stats.Memory.Usage,
		},
		{
			module: "memsw",
			entry:  stats.Memory.Swap,
		},
		{
			module: "kmem",
			entry:  stats.Memory.Kernel,
		},
		{
			module: "kmem.tcp",
			entry:  stats.Memory.KernelTCP,
		},
	} {
		for _, tt := range []struct {
			name  string
			value *uint64
		}{
			{
				name:  "usage_in_bytes",
				value: &t.entry.Usage,
			},
			{
				name:  "max_usage_in_bytes",
				value: &t.entry.Max,
			},
			{
				name:  "failcnt",
				value: &t.entry.Failcnt,
			},
			{
				name:  "limit_in_bytes",
				value: &t.entry.Limit,
			},
		} {
			parts := []string{"memory"}
			if t.module != "" {
				parts = append(parts, t.module)
			}
			parts = append(parts, tt.name)
			v, err := readUint(filepath.Join(m.Path(path), strings.Join(parts, ".")))
			if err != nil {
				return err
			}
			*tt.value = v
		}
	}
	return nil
}

func (m *memoryController) OOMEventFD(path string) (uintptr, error) {
	root := m.Path(path)
	f, err := os.Open(filepath.Join(root, "memory.oom_control"))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	fd, _, serr := unix.RawSyscall(unix.SYS_EVENTFD2, 0, unix.EFD_CLOEXEC, 0)
	if serr != 0 {
		return 0, serr
	}
	if err := writeEventFD(root, f.Fd(), fd); err != nil {
		unix.Close(int(fd))
		return 0, err
	}
	return fd, nil
}

func writeEventFD(root string, cfd, efd uintptr) error {
	f, err := os.OpenFile(filepath.Join(root, "cgroup.event_control"), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(fmt.Sprintf("%d %d", efd, cfd))
	f.Close()
	return err
}

func (m *memoryController) parseStats(r io.Reader, stat *MemoryStat) error {
	var (
		raw  = make(map[string]uint64)
		sc   = bufio.NewScanner(r)
		line int
	)
	for sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		key, v, err := parseKV(sc.Text())
		if err != nil {
			return fmt.Errorf("%d: %v", line, err)
		}
		raw[key] = v
		line++
	}
	stat.Cache = raw["cache"]
	stat.RSS = raw["rss"]
	stat.RSSHuge = raw["rss_huge"]
	stat.MappedFile = raw["mapped_file"]
	stat.Dirty = raw["dirty"]
	stat.Writeback = raw["writeback"]
	stat.PgPgIn = raw["pgpgin"]
	stat.PgPgOut = raw["pgpgout"]
	stat.PgFault = raw["pgfault"]
	stat.PgMajFault = raw["pgmajfault"]
	stat.InactiveAnon = raw["inactive_anon"]
	stat.ActiveAnon = raw["active_anon"]
	stat.InactiveFile = raw["inactive_file"]
	stat.ActiveFile = raw["active_file"]
	stat.Unevictable = raw["unevictable"]
	stat.HierarchicalMemoryLimit = raw["hierarchical_memory_limit"]
	stat.HierarchicalSwapLimit = raw["hierarchical_memsw_limit"]
	stat.TotalCache = raw["total_cache"]
	stat.TotalRSS = raw["total_rss"]
	stat.TotalRSSHuge = raw["total_rss_huge"]
	stat.TotalMappedFile = raw["total_mapped_file"]
	stat.TotalDirty = raw["total_dirty"]
	stat.TotalWriteback = raw["total_writeback"]
	stat.TotalPgPgIn = raw["total_pgpgin"]
	stat.TotalPgPgOut = raw["total_pgpgout"]
	stat.TotalPgFault = raw["total_pgfault"]
	stat.TotalPgMajFault = raw["total_pgmajfault"]
	stat.TotalInactiveAnon = raw["total_inactive_anon"]
	stat.TotalActiveAnon = raw["total_active_anon"]
	stat.TotalInactiveFile = raw["total_inactive_file"]
	stat.TotalActiveFile = raw["total_active_file"]
	stat.TotalUnevictable = raw["total_unevictable"]
	return nil
}

func (m *memoryController) set(path string, settings []memorySettings) error {
	for _, t := range settings {
		if t.value != nil {
			if err := ioutil.WriteFile(
				filepath.Join(m.Path(path), fmt.Sprintf("memory.%s", t.name)),
				[]byte(strconv.FormatInt(*t.value, 10)),
				defaultFilePerm,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

type memorySettings struct {
	name  string
	value *int64
}

func getMemorySettings(resources *specs.LinuxResources) []memorySettings {
	mem := resources.Memory
	var swappiness *int64
	if mem.Swappiness != nil {
		v := int64(*mem.Swappiness)
		swappiness = &v
	}
	return []memorySettings{
		{
			name:  "limit_in_bytes",
			value: mem.Limit,
		},
		{
			name: "soft_limit_in_bytes",
			value: mem.Reservation,
		},
		{
			name:  "memsw.limit_in_bytes",
			value: mem.Swap,
		},
		{
			name:  "kmem.limit_in_bytes",
			value: mem.Kernel,
		},
		{
			name:  "kmem.tcp.limit_in_bytes",
			value: mem.KernelTCP,
		},
		{
			name:  "oom_control",
			value: getOomControlValue(mem),
		},
		{
			name:  "swappiness",
			value: swappiness,
		},
	}
}

func checkEBUSY(err error) error {
	if pathErr, ok := err.(*os.PathError); ok {
		if errNo, ok := pathErr.Err.(syscall.Errno); ok {
			if errNo == unix.EBUSY {
				return fmt.Errorf(
					"failed to set memory.kmem.limit_in_bytes, because either tasks have already joined this cgroup or it has children")
			}
		}
	}
	return err
}

func getOomControlValue(mem *specs.LinuxMemory) *int64 {
	if mem.DisableOOMKiller != nil && *mem.DisableOOMKiller {
		i := int64(1)
		return &i
	}
	return nil
}
