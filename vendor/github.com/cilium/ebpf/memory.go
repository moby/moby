package ebpf

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/cilium/ebpf/internal/unix"
)

// Memory is the building block for accessing the memory of specific bpf map
// types (Array and Arena at the time of writing) without going through the bpf
// syscall interface.
//
// Given the fd of a bpf map created with the BPF_F_MMAPABLE flag, a shared
// 'file'-based memory-mapped region can be allocated in the process' address
// space, exposing the bpf map's memory by simply accessing a memory location.

var ErrReadOnly = errors.New("resource is read-only")

// Memory implements accessing a Map's memory without making any syscalls.
// Pay attention to the difference between Go and C struct alignment rules. Use
// [structs.HostLayout] on supported Go versions to help with alignment.
//
// Note on memory coherence: avoid using packed structs in memory shared between
// user space and eBPF C programs. This drops a struct's memory alignment to 1,
// forcing the compiler to use single-byte loads and stores for field accesses.
// This may lead to partially-written data to be observed from user space.
//
// On most architectures, the memmove implementation used by Go's copy() will
// access data in word-sized chunks. If paired with a matching access pattern on
// the eBPF C side (and if using default memory alignment), accessing shared
// memory without atomics or other synchronization primitives should be sound
// for individual values. For accesses beyond a single value, the usual
// concurrent programming rules apply.
type Memory struct {
	b  []byte
	ro bool
}

func newMemory(fd, size int) (*Memory, error) {
	// Typically, maps created with BPF_F_RDONLY_PROG remain writable from user
	// space until frozen. As a security precaution, the kernel doesn't allow
	// mapping bpf map memory as read-write into user space if the bpf map was
	// frozen, or if it was created using the RDONLY_PROG flag.
	//
	// The user would be able to write to the map after freezing (since the kernel
	// can't change the protection mode of an already-mapped page), while the
	// verifier assumes the contents to be immutable.
	b, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)

	// If the map is frozen when an rw mapping is requested, expect EPERM. If the
	// map was created with BPF_F_RDONLY_PROG, expect EACCES.
	var ro bool
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		ro = true
		b, err = unix.Mmap(fd, 0, size, unix.PROT_READ, unix.MAP_SHARED)
	}
	if err != nil {
		return nil, fmt.Errorf("setting up memory-mapped region: %w", err)
	}

	mm := &Memory{
		b,
		ro,
	}
	runtime.SetFinalizer(mm, (*Memory).close)

	return mm, nil
}

func (mm *Memory) close() {
	if err := unix.Munmap(mm.b); err != nil {
		panic(fmt.Errorf("unmapping memory: %w", err))
	}
	mm.b = nil
}

// Size returns the size of the memory-mapped region in bytes.
func (mm *Memory) Size() int {
	return len(mm.b)
}

// ReadOnly returns true if the memory-mapped region is read-only.
func (mm *Memory) ReadOnly() bool {
	return mm.ro
}

// bounds returns true if an access at off of the given size is within bounds.
func (mm *Memory) bounds(off uint64, size uint64) bool {
	return off+size < uint64(len(mm.b))
}

// ReadAt implements [io.ReaderAt]. Useful for creating a new [io.OffsetWriter].
//
// See [Memory] for details around memory coherence.
func (mm *Memory) ReadAt(p []byte, off int64) (int, error) {
	if mm.b == nil {
		return 0, fmt.Errorf("memory-mapped region closed")
	}

	if p == nil {
		return 0, fmt.Errorf("input buffer p is nil")
	}

	if off < 0 || off >= int64(len(mm.b)) {
		return 0, fmt.Errorf("read offset out of range")
	}

	n := copy(p, mm.b[off:])
	if n < len(p) {
		return n, io.EOF
	}

	return n, nil
}

// WriteAt implements [io.WriterAt]. Useful for creating a new
// [io.SectionReader].
//
// See [Memory] for details around memory coherence.
func (mm *Memory) WriteAt(p []byte, off int64) (int, error) {
	if mm.b == nil {
		return 0, fmt.Errorf("memory-mapped region closed")
	}
	if mm.ro {
		return 0, fmt.Errorf("memory-mapped region not writable: %w", ErrReadOnly)
	}

	if p == nil {
		return 0, fmt.Errorf("output buffer p is nil")
	}

	if off < 0 || off >= int64(len(mm.b)) {
		return 0, fmt.Errorf("write offset out of range")
	}

	n := copy(mm.b[off:], p)
	if n < len(p) {
		return n, io.EOF
	}

	return n, nil
}
