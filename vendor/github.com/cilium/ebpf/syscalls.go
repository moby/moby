package ebpf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/linux"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/tracefs"
	"github.com/cilium/ebpf/internal/unix"
)

var (
	// pre-allocating these here since they may
	// get called in hot code paths and cause
	// unnecessary memory allocations
	sysErrKeyNotExist  = sys.Error(ErrKeyNotExist, unix.ENOENT)
	sysErrKeyExist     = sys.Error(ErrKeyExist, unix.EEXIST)
	sysErrNotSupported = sys.Error(ErrNotSupported, sys.ENOTSUPP)
)

// invalidBPFObjNameChar returns true if char may not appear in
// a BPF object name.
func invalidBPFObjNameChar(char rune) bool {
	dotAllowed := objNameAllowsDot() == nil

	switch {
	case char >= 'A' && char <= 'Z':
		return false
	case char >= 'a' && char <= 'z':
		return false
	case char >= '0' && char <= '9':
		return false
	case dotAllowed && char == '.':
		return false
	case char == '_':
		return false
	default:
		return true
	}
}

func progLoad(insns asm.Instructions, typ ProgramType, license string) (*sys.FD, error) {
	buf := bytes.NewBuffer(make([]byte, 0, insns.Size()))
	if err := insns.Marshal(buf, internal.NativeEndian); err != nil {
		return nil, err
	}
	bytecode := buf.Bytes()

	return sys.ProgLoad(&sys.ProgLoadAttr{
		ProgType: sys.ProgType(typ),
		License:  sys.NewStringPointer(license),
		Insns:    sys.NewSlicePointer(bytecode),
		InsnCnt:  uint32(len(bytecode) / asm.InstructionSize),
	})
}

var haveNestedMaps = internal.NewFeatureTest("nested maps", func() error {
	_, err := sys.MapCreate(&sys.MapCreateAttr{
		MapType:    sys.MapType(ArrayOfMaps),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		// Invalid file descriptor.
		InnerMapFd: ^uint32(0),
	})
	if errors.Is(err, unix.EINVAL) {
		return internal.ErrNotSupported
	}
	if errors.Is(err, unix.EBADF) {
		return nil
	}
	return err
}, "4.12")

var haveMapMutabilityModifiers = internal.NewFeatureTest("read- and write-only maps", func() error {
	// This checks BPF_F_RDONLY_PROG and BPF_F_WRONLY_PROG. Since
	// BPF_MAP_FREEZE appeared in 5.2 as well we don't do a separate check.
	m, err := sys.MapCreate(&sys.MapCreateAttr{
		MapType:    sys.MapType(Array),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		MapFlags:   sys.BPF_F_RDONLY_PROG,
	})
	if err != nil {
		return internal.ErrNotSupported
	}
	_ = m.Close()
	return nil
}, "5.2")

var haveMmapableMaps = internal.NewFeatureTest("mmapable maps", func() error {
	// This checks BPF_F_MMAPABLE, which appeared in 5.5 for array maps.
	m, err := sys.MapCreate(&sys.MapCreateAttr{
		MapType:    sys.MapType(Array),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		MapFlags:   sys.BPF_F_MMAPABLE,
	})
	if err != nil {
		return internal.ErrNotSupported
	}
	_ = m.Close()
	return nil
}, "5.5")

var haveInnerMaps = internal.NewFeatureTest("inner maps", func() error {
	// This checks BPF_F_INNER_MAP, which appeared in 5.10.
	m, err := sys.MapCreate(&sys.MapCreateAttr{
		MapType:    sys.MapType(Array),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		MapFlags:   sys.BPF_F_INNER_MAP,
	})

	if err != nil {
		return internal.ErrNotSupported
	}
	_ = m.Close()
	return nil
}, "5.10")

var haveNoPreallocMaps = internal.NewFeatureTest("prealloc maps", func() error {
	// This checks BPF_F_NO_PREALLOC, which appeared in 4.6.
	m, err := sys.MapCreate(&sys.MapCreateAttr{
		MapType:    sys.MapType(Hash),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		MapFlags:   sys.BPF_F_NO_PREALLOC,
	})

	if err != nil {
		return internal.ErrNotSupported
	}
	_ = m.Close()
	return nil
}, "4.6")

func wrapMapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, unix.ENOENT) {
		return sysErrKeyNotExist
	}

	if errors.Is(err, unix.EEXIST) {
		return sysErrKeyExist
	}

	if errors.Is(err, sys.ENOTSUPP) {
		return sysErrNotSupported
	}

	if errors.Is(err, unix.E2BIG) {
		return fmt.Errorf("key too big for map: %w", err)
	}

	return err
}

var haveObjName = internal.NewFeatureTest("object names", func() error {
	attr := sys.MapCreateAttr{
		MapType:    sys.MapType(Array),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		MapName:    sys.NewObjName("feature_test"),
	}

	// Tolerate EPERM as this runs during ELF loading which is potentially
	// unprivileged. Only EINVAL is conclusive, thrown from CHECK_ATTR.
	fd, err := sys.MapCreate(&attr)
	if errors.Is(err, unix.EPERM) {
		return nil
	}
	if errors.Is(err, unix.EINVAL) {
		return internal.ErrNotSupported
	}
	if err != nil {
		return err
	}

	_ = fd.Close()
	return nil
}, "4.15")

var objNameAllowsDot = internal.NewFeatureTest("dot in object names", func() error {
	if err := haveObjName(); err != nil {
		return err
	}

	attr := sys.MapCreateAttr{
		MapType:    sys.MapType(Array),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
		MapName:    sys.NewObjName(".test"),
	}

	// Tolerate EPERM, otherwise MapSpec.Name has its dots removed when run by
	// unprivileged tools. (bpf2go, other code gen). Only EINVAL is conclusive,
	// thrown from bpf_obj_name_cpy().
	fd, err := sys.MapCreate(&attr)
	if errors.Is(err, unix.EPERM) {
		return nil
	}
	if errors.Is(err, unix.EINVAL) {
		return internal.ErrNotSupported
	}
	if err != nil {
		return err
	}

	_ = fd.Close()
	return nil
}, "5.2")

var haveBatchAPI = internal.NewFeatureTest("map batch api", func() error {
	var maxEntries uint32 = 2
	attr := sys.MapCreateAttr{
		MapType:    sys.MapType(Hash),
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: maxEntries,
	}

	fd, err := sys.MapCreate(&attr)
	if err != nil {
		return internal.ErrNotSupported
	}
	defer fd.Close()

	keys := []uint32{1, 2}
	values := []uint32{3, 4}
	kp, _ := marshalMapSyscallInput(keys, 8)
	vp, _ := marshalMapSyscallInput(values, 8)

	err = sys.MapUpdateBatch(&sys.MapUpdateBatchAttr{
		MapFd:  fd.Uint(),
		Keys:   kp,
		Values: vp,
		Count:  maxEntries,
	})
	if err != nil {
		return internal.ErrNotSupported
	}
	return nil
}, "5.6")

var haveProbeReadKernel = internal.NewFeatureTest("bpf_probe_read_kernel", func() error {
	insns := asm.Instructions{
		asm.Mov.Reg(asm.R1, asm.R10),
		asm.Add.Imm(asm.R1, -8),
		asm.Mov.Imm(asm.R2, 8),
		asm.Mov.Imm(asm.R3, 0),
		asm.FnProbeReadKernel.Call(),
		asm.Return(),
	}

	fd, err := progLoad(insns, Kprobe, "GPL")
	if err != nil {
		return internal.ErrNotSupported
	}
	_ = fd.Close()
	return nil
}, "5.5")

var haveBPFToBPFCalls = internal.NewFeatureTest("bpf2bpf calls", func() error {
	insns := asm.Instructions{
		asm.Call.Label("prog2").WithSymbol("prog1"),
		asm.Return(),
		asm.Mov.Imm(asm.R0, 0).WithSymbol("prog2"),
		asm.Return(),
	}

	fd, err := progLoad(insns, SocketFilter, "MIT")
	if err != nil {
		return internal.ErrNotSupported
	}
	_ = fd.Close()
	return nil
}, "4.16")

var haveSyscallWrapper = internal.NewFeatureTest("syscall wrapper", func() error {
	prefix := linux.PlatformPrefix()
	if prefix == "" {
		return fmt.Errorf("unable to find the platform prefix for (%s)", runtime.GOARCH)
	}

	args := tracefs.ProbeArgs{
		Type:   tracefs.Kprobe,
		Symbol: prefix + "sys_bpf",
		Pid:    -1,
	}

	var err error
	args.Group, err = tracefs.RandomGroup("ebpf_probe")
	if err != nil {
		return err
	}

	evt, err := tracefs.NewEvent(args)
	if errors.Is(err, os.ErrNotExist) {
		return internal.ErrNotSupported
	}
	if err != nil {
		return err
	}

	return evt.Close()
}, "4.17")

var haveProgramExtInfos = internal.NewFeatureTest("program ext_infos", func() error {
	insns := asm.Instructions{
		asm.Mov.Imm(asm.R0, 0),
		asm.Return(),
	}

	buf := bytes.NewBuffer(make([]byte, 0, insns.Size()))
	if err := insns.Marshal(buf, internal.NativeEndian); err != nil {
		return err
	}
	bytecode := buf.Bytes()

	_, err := sys.ProgLoad(&sys.ProgLoadAttr{
		ProgType:    sys.ProgType(SocketFilter),
		License:     sys.NewStringPointer("MIT"),
		Insns:       sys.NewSlicePointer(bytecode),
		InsnCnt:     uint32(len(bytecode) / asm.InstructionSize),
		FuncInfoCnt: 1,
		ProgBtfFd:   math.MaxUint32,
	})

	if errors.Is(err, unix.EBADF) {
		return nil
	}

	if errors.Is(err, unix.E2BIG) {
		return ErrNotSupported
	}

	return err
}, "5.0")
