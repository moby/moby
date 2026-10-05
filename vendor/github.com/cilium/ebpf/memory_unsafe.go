package ebpf

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/unix"
)

// This file contains an experimental, unsafe implementation of Memory that
// allows taking a Go pointer to a memory-mapped region. This currently does not
// have first-class support from the Go runtime, so it may break in future Go
// versions. The Go proposal for the runtime to track off-heap pointers is here:
// https://github.com/golang/go/issues/70224.
//
// In Go, the programmer should not have to worry about freeing memory. Since
// this API synthesizes Go variables around global variables declared in a BPF
// C program, we want to lean on the runtime for making sure accessing them is
// safe at all times. Unfortunately, Go (as of 1.24) does not have the ability
// of automatically managing memory that was not allocated by the runtime.
//
// This led to a solution that requests regular Go heap memory by allocating a
// slice (making the runtime track pointers into the slice's backing array) and
// memory-mapping the bpf map's memory over it. Then, before returning the
// Memory to the caller, a finalizer is set on the backing array, making sure
// the bpf map's memory is unmapped from the heap before releasing the backing
// array to the runtime for reallocation.
//
// This obviates the need to maintain a reference to the *Memory at all times,
// which is difficult for the caller to achieve if the variable access is done
// through another object (like a sync.Atomic) that can potentially be passed
// around the Go application. Accidentally losing the reference to the *Memory
// would result in hard-to-debug segfaults, which are always unexpected in Go.

//go:linkname heapObjectsCanMove runtime.heapObjectsCanMove
func heapObjectsCanMove() bool

// Set from a file behind the ebpf_unsafe_memory_experiment build tag to enable
// features that require mapping bpf map memory over the Go heap.
var unsafeMemory = false

// ErrInvalidType is returned when the given type cannot be used as a Memory or
// Variable pointer.
var ErrInvalidType = errors.New("invalid type")

func newUnsafeMemory(fd, size int) (*Memory, error) {
	// Some architectures need the size to be page-aligned to work with MAP_FIXED.
	if size%os.Getpagesize() != 0 {
		return nil, fmt.Errorf("memory: must be a multiple of page size (requested %d bytes)", size)
	}

	// Allocate a page-aligned span of memory on the Go heap.
	alloc, err := allocate(size)
	if err != nil {
		return nil, fmt.Errorf("allocating memory: %w", err)
	}

	// Typically, maps created with BPF_F_RDONLY_PROG remain writable from user
	// space until frozen. As a security precaution, the kernel doesn't allow
	// mapping bpf map memory as read-write into user space if the bpf map was
	// frozen, or if it was created using the RDONLY_PROG flag.
	//
	// The user would be able to write to the map after freezing (since the kernel
	// can't change the protection mode of an already-mapped page), while the
	// verifier assumes the contents to be immutable.
	//
	// Map the bpf map memory over a page-aligned allocation on the Go heap.
	err = mapmap(fd, alloc, size, unix.PROT_READ|unix.PROT_WRITE)

	// If the map is frozen when an rw mapping is requested, expect EPERM. If the
	// map was created with BPF_F_RDONLY_PROG, expect EACCES.
	var ro bool
	if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		ro = true
		err = mapmap(fd, alloc, size, unix.PROT_READ)
	}
	if err != nil {
		return nil, fmt.Errorf("setting up memory-mapped region: %w", err)
	}

	mm := &Memory{
		unsafe.Slice((*byte)(alloc), size),
		ro,
		true,
		runtime.Cleanup{},
	}

	return mm, nil
}

// allocate returns a pointer to a page-aligned section of memory on the Go
// heap, managed by the runtime.
//
//go:nocheckptr
func allocate(size int) (unsafe.Pointer, error) {
	// Memory-mapping over a piece of the Go heap is unsafe when the GC can
	// randomly decide to move objects around, in which case the mapped region
	// will not move along with it.
	if heapObjectsCanMove() {
		return nil, errors.New("this Go runtime has a moving garbage collector")
	}

	if size == 0 {
		return nil, errors.New("size must be greater than 0")
	}

	// Request at least two pages of memory from the runtime to ensure we can
	// align the requested allocation to a page boundary. This is needed for
	// MAP_FIXED and makes sure we don't mmap over some other allocation on the Go
	// heap.
	size = internal.Align(size+os.Getpagesize(), os.Getpagesize())

	// Allocate a new slice and store a pointer to its backing array.
	alloc := unsafe.Pointer(unsafe.SliceData(make([]byte, size)))

	// Align the pointer to a page boundary within the allocation. This may alias
	// the initial pointer if it was already page-aligned.
	aligned := internal.Align(uintptr(alloc), uintptr(os.Getpagesize()))
	alloc = unsafe.Add(alloc, aligned-uintptr(alloc))

	// Return an aligned pointer into the backing array, losing the original
	// reference. The runtime.SetFinalizer docs specify that its argument 'must be
	// a pointer to an object, complit or local var', but this is still somewhat
	// vague and not enforced by the current implementation.
	//
	// Currently, finalizers can be set and triggered from any address within a
	// heap allocation, even individual struct fields or arbitrary offsets within
	// a slice. In this case, finalizers set on struct fields or slice offsets
	// will only run when the whole struct or backing array are collected. The
	// accepted runtime.AddCleanup proposal makes this behaviour more explicit and
	// is set to deprecate runtime.SetFinalizer.
	//
	// Alternatively, we'd have to track the original allocation and the aligned
	// pointer separately, which severely complicates finalizer setup and makes it
	// prone to human error. For now, just bump the pointer and treat it as the
	// new and only reference to the backing array.
	return alloc, nil
}

// mapmap memory-maps the given file descriptor at the given address and sets a
// finalizer on addr to unmap it when it's no longer reachable.
func mapmap(fd int, addr unsafe.Pointer, size, flags int) error {
	// Map the bpf map memory over the Go heap. This will result in the following
	// mmap layout in the process' address space (0xc000000000 is a span of Go
	// heap), visualized using pmap:
	//
	// Address           Kbytes     RSS   Dirty Mode  Mapping
	// 000000c000000000    1824     864     864 rw--- [ anon ]
	// 000000c0001c8000       4       4       4 rw-s- [ anon ]
	// 000000c0001c9000    2268      16      16 rw--- [ anon ]
	//
	// This will break up the Go heap, but as long as the runtime doesn't try to
	// move our allocation around, this is safe for as long as we hold a reference
	// to our allocated object.
	//
	// Use MAP_SHARED to make sure the kernel sees any writes we do, and MAP_FIXED
	// to ensure the mapping starts exactly at the address we requested. If alloc
	// isn't page-aligned, the mapping operation will fail.
	if _, err := unix.MmapPtr(fd, 0, addr, uintptr(size),
		flags, unix.MAP_SHARED|unix.MAP_FIXED); err != nil {
		return fmt.Errorf("setting up memory-mapped region: %w", err)
	}

	// Set a finalizer on the heap allocation to undo the mapping before the span
	// is collected and reused by the runtime. This has a few reasons:
	//
	//  - Avoid leaking memory/mappings.
	//  - Future writes to this memory should never clobber a bpf map's contents.
	//  - Some bpf maps are mapped read-only, causing a segfault if the runtime
	//    reallocates and zeroes the span later.
	runtime.SetFinalizer((*byte)(addr), unmap(size))

	return nil
}

// unmap returns a function that takes a pointer to a memory-mapped region on
// the Go heap. The function undoes any mappings and discards the span's
// contents.
//
// Used as a finalizer in [newMemory], split off into a separate function for
// testing and to avoid accidentally closing over the unsafe.Pointer to the
// memory region, which would cause a cyclical reference.
//
// The resulting function panics if the mmap operation returns an error, since
// it would mean the integrity of the Go heap is compromised.
func unmap(size int) func(*byte) {
	return func(a *byte) {
		// Create another mapping at the same address to undo the original mapping.
		// This will cause the kernel to repair the slab since we're using the same
		// protection mode and flags as the original mapping for the Go heap.
		//
		// Address           Kbytes     RSS   Dirty Mode  Mapping
		// 000000c000000000    4096     884     884 rw--- [ anon ]
		//
		// Using munmap here would leave an unmapped hole in the heap, compromising
		// its integrity.
		//
		// MmapPtr allocates another unsafe.Pointer at the same address. Even though
		// we discard it here, it may temporarily resurrect the backing array and
		// delay its collection to the next GC cycle.
		_, err := unix.MmapPtr(-1, 0, unsafe.Pointer(a), uintptr(size),
			unix.PROT_READ|unix.PROT_WRITE,
			unix.MAP_PRIVATE|unix.MAP_FIXED|unix.MAP_ANON)
		if err != nil {
			panic(fmt.Errorf("undoing bpf map memory mapping: %w", err))
		}
	}
}

// checkUnsafeMemory ensures value T can be accessed in mm at offset off.
//
// The comparable constraint narrows down the set of eligible types to exclude
// slices, maps and functions. These complex types cannot be mapped to memory
// directly.
func checkUnsafeMemory[T comparable](mm *Memory, off uint32) error {
	if mm.b == nil {
		return fmt.Errorf("memory-mapped region is nil")
	}
	if mm.ro {
		return ErrReadOnly
	}
	if !mm.heap {
		return fmt.Errorf("memory region is not heap-mapped, build with '-tags ebpf_unsafe_memory_experiment' to enable: %w", ErrNotSupported)
	}

	t := reflect.TypeFor[T]()
	if err := checkType(t.String(), t); err != nil {
		return err
	}

	size := t.Size()
	if size == 0 {
		return fmt.Errorf("zero-sized type %s: %w", t, ErrInvalidType)
	}

	if off%uint32(t.Align()) != 0 {
		return fmt.Errorf("unaligned access of memory-mapped region: %d-byte aligned read at offset %d", t.Align(), off)
	}

	vs, bs := uint32(size), uint32(len(mm.b))
	if !mm.bounds(off, vs) {
		return fmt.Errorf("%d-byte value at offset %d exceeds mmap size of %d bytes", vs, off, bs)
	}

	return nil
}

// checkType recursively checks if the given type is supported for memory
// mapping. Only fixed-size, non-Go-pointer types are supported: bools, floats,
// (u)int[8-64], arrays, and structs containing them. As an exception, uintptr
// is allowed since the backing memory is expected to contain 32-bit pointers on
// 32-bit systems despite BPF always allocating 64 bits for pointers in a data
// section.
//
// Doesn't check for loops since it rejects pointers. Should that ever change, a
// visited set would be needed.
func checkType(name string, t reflect.Type) error {
	// Special-case atomic types to allow them to be used as root types as well as
	// struct fields. Notably, omit atomic.Value and atomic.Pointer since those
	// are pointer types. Also, atomic.Value embeds an interface value, which
	// doesn't make sense to share with C land.
	if t.PkgPath() == "sync/atomic" {
		switch t.Name() {
		case "Bool", "Int32", "Int64", "Uint32", "Uint64", "Uintptr":
			return nil
		}
	}

	switch t.Kind() {
	case reflect.Uintptr, reflect.Bool, reflect.Float32, reflect.Float64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return nil

	case reflect.Array:
		at := t.Elem()
		if err := checkType(fmt.Sprintf("%s.%s", name, at.String()), at); err != nil {
			return err
		}

	case reflect.Struct:
		var hasHostLayout bool
		for i := range t.NumField() {
			at := t.Field(i).Type

			// Require [structs.HostLayout] to be embedded in all structs. Check the
			// full package path to reject a user-defined HostLayout type.
			if at.PkgPath() == "structs" && at.Name() == "HostLayout" {
				hasHostLayout = true
				continue
			}

			if err := checkType(fmt.Sprintf("%s.%s", name, at.String()), at); err != nil {
				return err
			}
		}

		if !hasHostLayout {
			return fmt.Errorf("struct %s must embed structs.HostLayout: %w", name, ErrInvalidType)
		}

	default:
		// For basic types like int and bool, the kind name is the same as the type
		// name, so the fallthrough case would print 'int type int not supported'.
		// Omit the kind name if it matches the type name.
		if t.String() == t.Kind().String() {
			// Output: type int not supported
			return fmt.Errorf("type %s not supported: %w", name, ErrInvalidType)
		}

		// Output: interface value io.Reader not supported
		return fmt.Errorf("%s type %s not supported: %w", t.Kind(), name, ErrInvalidType)
	}

	return nil
}

// memoryPointer returns a pointer to a value of type T at offset off in mm.
// Taking a pointer to a read-only Memory or to a Memory that is not heap-mapped
// is not supported.
//
// T must contain only fixed-size, non-Go-pointer types: bools, floats,
// (u)int[8-64], arrays, and structs containing them. Structs must embed
// [structs.HostLayout]. [ErrInvalidType] is returned if T is not a valid type.
//
// Memory must be writable, off must be aligned to the size of T, and the value
// must be within bounds of the Memory.
//
// To access read-only memory, use [Memory.ReadAt].
func memoryPointer[T comparable](mm *Memory, off uint32) (*T, error) {
	if err := checkUnsafeMemory[T](mm, off); err != nil {
		return nil, fmt.Errorf("memory pointer: %w", err)
	}
	return (*T)(unsafe.Pointer(&mm.b[off])), nil
}
