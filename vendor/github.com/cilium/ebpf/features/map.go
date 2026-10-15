package features

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// HaveMapType probes the running kernel for the availability of the specified map type.
//
// See the package documentation for the meaning of the error return value.
func HaveMapType(mt ebpf.MapType) error {
	return haveMapTypeMatrix.Result(mt)
}

func probeCgroupStorageMap(mt sys.MapType) error {
	// keySize needs to be sizeof(struct{u32 + u64}) = 12 (+ padding = 16)
	// by using unsafe.Sizeof(int) we are making sure that this works on 32bit and 64bit archs
	return createMap(&sys.MapCreateAttr{
		MapType:    mt,
		ValueSize:  4,
		KeySize:    uint32(8 + unsafe.Sizeof(int(0))),
		MaxEntries: 0,
	})
}

func probeStorageMap(mt sys.MapType) error {
	// maxEntries needs to be 0
	// BPF_F_NO_PREALLOC needs to be set
	// btf* fields need to be set
	// see alloc_check for local_storage map types
	err := createMap(&sys.MapCreateAttr{
		MapType:        mt,
		KeySize:        4,
		ValueSize:      4,
		MaxEntries:     0,
		MapFlags:       sys.BPF_F_NO_PREALLOC,
		BtfKeyTypeId:   1,
		BtfValueTypeId: 1,
		BtfFd:          ^uint32(0),
	})
	if errors.Is(err, unix.EBADF) {
		// Triggered by BtfFd.
		return nil
	}
	return err
}

func probeNestedMap(mt sys.MapType) error {
	// assign invalid innerMapFd to pass validation check
	// will return EBADF
	err := probeMap(&sys.MapCreateAttr{
		MapType:    mt,
		InnerMapFd: ^uint32(0),
	})
	if errors.Is(err, unix.EBADF) {
		return nil
	}
	return err
}

func probeMap(attr *sys.MapCreateAttr) error {
	if attr.KeySize == 0 {
		attr.KeySize = 4
	}
	if attr.ValueSize == 0 {
		attr.ValueSize = 4
	}
	attr.MaxEntries = 1
	return createMap(attr)
}

func createMap(attr *sys.MapCreateAttr) error {
	fd, err := sys.MapCreate(attr)
	if err == nil {
		fd.Close()
		return nil
	}

	switch {
	// EINVAL occurs when attempting to create a map with an unknown type.
	// E2BIG occurs when MapCreateAttr contains non-zero bytes past the end
	// of the struct known by the running kernel, meaning the kernel is too old
	// to support the given map type.
	case errors.Is(err, unix.EINVAL), errors.Is(err, unix.E2BIG):
		return ebpf.ErrNotSupported
	}

	return err
}

var haveMapTypeMatrix = internal.FeatureMatrix[ebpf.MapType]{
	ebpf.Hash:           {Version: "3.19"},
	ebpf.Array:          {Version: "3.19"},
	ebpf.ProgramArray:   {Version: "4.2"},
	ebpf.PerfEventArray: {Version: "4.3"},
	ebpf.PerCPUHash:     {Version: "4.6"},
	ebpf.PerCPUArray:    {Version: "4.6"},
	ebpf.StackTrace: {
		Version: "4.6",
		Fn: func() error {
			return probeMap(&sys.MapCreateAttr{
				MapType:   sys.BPF_MAP_TYPE_STACK_TRACE,
				ValueSize: 8, // sizeof(uint64)
			})
		},
	},
	ebpf.CGroupArray: {Version: "4.8"},
	ebpf.LRUHash:     {Version: "4.10"},
	ebpf.LRUCPUHash:  {Version: "4.10"},
	ebpf.LPMTrie: {
		Version: "4.11",
		Fn: func() error {
			// keySize and valueSize need to be sizeof(struct{u32 + u8}) + 1 + padding = 8
			// BPF_F_NO_PREALLOC needs to be set
			return probeMap(&sys.MapCreateAttr{
				MapType:   sys.BPF_MAP_TYPE_LPM_TRIE,
				KeySize:   8,
				ValueSize: 8,
				MapFlags:  sys.BPF_F_NO_PREALLOC,
			})
		},
	},
	ebpf.ArrayOfMaps: {
		Version: "4.12",
		Fn:      func() error { return probeNestedMap(sys.BPF_MAP_TYPE_ARRAY_OF_MAPS) },
	},
	ebpf.HashOfMaps: {
		Version: "4.12",
		Fn:      func() error { return probeNestedMap(sys.BPF_MAP_TYPE_HASH_OF_MAPS) },
	},
	ebpf.DevMap:   {Version: "4.14"},
	ebpf.SockMap:  {Version: "4.14"},
	ebpf.CPUMap:   {Version: "4.15"},
	ebpf.XSKMap:   {Version: "4.18"},
	ebpf.SockHash: {Version: "4.18"},
	ebpf.CGroupStorage: {
		Version: "4.19",
		Fn:      func() error { return probeCgroupStorageMap(sys.BPF_MAP_TYPE_CGROUP_STORAGE) },
	},
	ebpf.ReusePortSockArray: {Version: "4.19"},
	ebpf.PerCPUCGroupStorage: {
		Version: "4.20",
		Fn:      func() error { return probeCgroupStorageMap(sys.BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE) },
	},
	ebpf.Queue: {
		Version: "4.20",
		Fn: func() error {
			return createMap(&sys.MapCreateAttr{
				MapType:    sys.BPF_MAP_TYPE_QUEUE,
				KeySize:    0,
				ValueSize:  4,
				MaxEntries: 1,
			})
		},
	},
	ebpf.Stack: {
		Version: "4.20",
		Fn: func() error {
			return createMap(&sys.MapCreateAttr{
				MapType:    sys.BPF_MAP_TYPE_STACK,
				KeySize:    0,
				ValueSize:  4,
				MaxEntries: 1,
			})
		},
	},
	ebpf.SkStorage: {
		Version: "5.2",
		Fn:      func() error { return probeStorageMap(sys.BPF_MAP_TYPE_SK_STORAGE) },
	},
	ebpf.DevMapHash: {Version: "5.4"},
	ebpf.StructOpsMap: {
		Version: "5.6",
		Fn: func() error {
			// StructOps requires setting a vmlinux type id, but id 1 will always
			// resolve to some type of integer. This will cause ENOTSUPP.
			err := probeMap(&sys.MapCreateAttr{
				MapType:               sys.BPF_MAP_TYPE_STRUCT_OPS,
				BtfVmlinuxValueTypeId: 1,
			})
			if errors.Is(err, sys.ENOTSUPP) {
				// ENOTSUPP means the map type is at least known to the kernel.
				return nil
			}
			return err
		},
	},
	ebpf.RingBuf: {
		Version: "5.8",
		Fn: func() error {
			// keySize and valueSize need to be 0
			// maxEntries needs to be power of 2 and PAGE_ALIGNED
			return createMap(&sys.MapCreateAttr{
				MapType:    sys.BPF_MAP_TYPE_RINGBUF,
				KeySize:    0,
				ValueSize:  0,
				MaxEntries: uint32(os.Getpagesize()),
			})
		},
	},
	ebpf.InodeStorage: {
		Version: "5.10",
		Fn:      func() error { return probeStorageMap(sys.BPF_MAP_TYPE_INODE_STORAGE) },
	},
	ebpf.TaskStorage: {
		Version: "5.11",
		Fn:      func() error { return probeStorageMap(sys.BPF_MAP_TYPE_TASK_STORAGE) },
	},
	ebpf.BloomFilter: {
		Version: "5.16",
		Fn: func() error {
			return createMap(&sys.MapCreateAttr{
				MapType:    sys.BPF_MAP_TYPE_BLOOM_FILTER,
				KeySize:    0,
				ValueSize:  4,
				MaxEntries: 1,
			})
		},
	},
	ebpf.UserRingbuf: {
		Version: "6.1",
		Fn: func() error {
			// keySize and valueSize need to be 0
			// maxEntries needs to be power of 2 and PAGE_ALIGNED
			return createMap(&sys.MapCreateAttr{
				MapType:    sys.BPF_MAP_TYPE_USER_RINGBUF,
				KeySize:    0,
				ValueSize:  0,
				MaxEntries: uint32(os.Getpagesize()),
			})
		},
	},
	ebpf.CgroupStorage: {
		Version: "6.2",
		Fn:      func() error { return probeStorageMap(sys.BPF_MAP_TYPE_CGRP_STORAGE) },
	},
	ebpf.Arena: {
		Version: "6.9",
		Fn: func() error {
			return createMap(&sys.MapCreateAttr{
				MapType:    sys.BPF_MAP_TYPE_ARENA,
				KeySize:    0,
				ValueSize:  0,
				MaxEntries: 1, // one page
				MapExtra:   0, // can mmap() at any address
				MapFlags:   sys.BPF_F_MMAPABLE,
			})
		},
	},
}

func init() {
	for mt, ft := range haveMapTypeMatrix {
		ft.Name = mt.String()
		if ft.Fn == nil {
			// Avoid referring to the loop variable in the closure.
			mt := sys.MapType(mt)
			ft.Fn = func() error { return probeMap(&sys.MapCreateAttr{MapType: mt}) }
		}
	}
}

// MapFlags document which flags may be feature probed.
type MapFlags uint32

// Flags which may be feature probed.
const (
	BPF_F_NO_PREALLOC = sys.BPF_F_NO_PREALLOC
	BPF_F_RDONLY_PROG = sys.BPF_F_RDONLY_PROG
	BPF_F_WRONLY_PROG = sys.BPF_F_WRONLY_PROG
	BPF_F_MMAPABLE    = sys.BPF_F_MMAPABLE
	BPF_F_INNER_MAP   = sys.BPF_F_INNER_MAP
)

// HaveMapFlag probes the running kernel for the availability of the specified map flag.
//
// Returns an error if flag is not one of the flags declared in this package.
// See the package documentation for the meaning of the error return value.
func HaveMapFlag(flag MapFlags) (err error) {
	return haveMapFlagsMatrix.Result(flag)
}

func probeMapFlag(attr *sys.MapCreateAttr) error {
	// For now, we do not check if the map type is supported because we only support
	// probing for flags defined on arrays and hashes that are always supported.
	// In the future, if we allow probing on flags defined on newer types, checking for map type
	// support will be required.
	if attr.MapType == sys.BPF_MAP_TYPE_UNSPEC {
		attr.MapType = sys.BPF_MAP_TYPE_ARRAY
	}

	attr.KeySize = 4
	attr.ValueSize = 4
	attr.MaxEntries = 1

	fd, err := sys.MapCreate(attr)
	if err == nil {
		fd.Close()
	} else if errors.Is(err, unix.EINVAL) {
		// EINVAL occurs when attempting to create a map with an unknown type or an unknown flag.
		err = ebpf.ErrNotSupported
	}

	return err
}

var haveMapFlagsMatrix = internal.FeatureMatrix[MapFlags]{
	BPF_F_NO_PREALLOC: {
		Version: "4.6",
		Fn: func() error {
			return probeMapFlag(&sys.MapCreateAttr{
				MapType:  sys.BPF_MAP_TYPE_HASH,
				MapFlags: BPF_F_NO_PREALLOC,
			})
		},
	},
	BPF_F_RDONLY_PROG: {
		Version: "5.2",
		Fn: func() error {
			return probeMapFlag(&sys.MapCreateAttr{
				MapFlags: BPF_F_RDONLY_PROG,
			})
		},
	},
	BPF_F_WRONLY_PROG: {
		Version: "5.2",
		Fn: func() error {
			return probeMapFlag(&sys.MapCreateAttr{
				MapFlags: BPF_F_WRONLY_PROG,
			})
		},
	},
	BPF_F_MMAPABLE: {
		Version: "5.5",
		Fn: func() error {
			return probeMapFlag(&sys.MapCreateAttr{
				MapFlags: BPF_F_MMAPABLE,
			})
		},
	},
	BPF_F_INNER_MAP: {
		Version: "5.10",
		Fn: func() error {
			return probeMapFlag(&sys.MapCreateAttr{
				MapFlags: BPF_F_INNER_MAP,
			})
		},
	},
}

func init() {
	for mf, ft := range haveMapFlagsMatrix {
		ft.Name = fmt.Sprint(mf)
	}
}
