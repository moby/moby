package ebpf

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// The *Info structs expose metadata about a program or map. Most
// fields are exposed via a getter:
//
//     func (*MapInfo) ID() (MapID, bool)
//
// This is because the metadata available changes based on kernel version.
// The second boolean return value indicates whether a particular field is
// available on the current kernel.
//
// Always add new metadata as such a getter, unless you can somehow get the
// value of the field on all supported kernels. Also document which version
// a particular field first appeared in.
//
// Some metadata is a buffer which needs additional parsing. In this case,
// store the undecoded data in the Info struct and provide a getter which
// decodes it when necessary. See ProgramInfo.Instructions for an example.

// MapInfo describes a map.
type MapInfo struct {
	// Type of the map.
	Type MapType
	// KeySize is the size of the map key in bytes.
	KeySize uint32
	// ValueSize is the size of the map value in bytes.
	ValueSize uint32
	// MaxEntries is the maximum number of entries the map can hold. Its meaning
	// is map-specific.
	MaxEntries uint32
	// Flags used during map creation.
	Flags uint32
	// Name as supplied by user space at load time. Available from 4.15.
	Name string

	id       MapID
	btf      btf.ID
	mapExtra uint64
	memlock  uint64
	frozen   bool
}

// newMapInfoFromFd queries map information about the given fd. [sys.ObjInfo] is
// attempted first, supplementing any missing values with information from
// /proc/self/fdinfo. Ignores EINVAL from ObjInfo as well as ErrNotSupported
// from reading fdinfo (indicating the file exists, but no fields of interest
// were found). If both fail, an error is always returned.
func newMapInfoFromFd(fd *sys.FD) (*MapInfo, error) {
	var info sys.MapInfo
	err1 := sys.ObjInfo(fd, &info)
	// EINVAL means the kernel doesn't support BPF_OBJ_GET_INFO_BY_FD. Continue
	// with fdinfo if that's the case.
	if err1 != nil && !errors.Is(err1, unix.EINVAL) {
		return nil, fmt.Errorf("getting object info: %w", err1)
	}

	mi := &MapInfo{
		MapType(info.Type),
		info.KeySize,
		info.ValueSize,
		info.MaxEntries,
		uint32(info.MapFlags),
		unix.ByteSliceToString(info.Name[:]),
		MapID(info.Id),
		btf.ID(info.BtfId),
		info.MapExtra,
		0,
		false,
	}

	// Supplement OBJ_INFO with data from /proc/self/fdinfo. It contains fields
	// like memlock and frozen that are not present in OBJ_INFO.
	err2 := readMapInfoFromProc(fd, mi)
	if err2 != nil && !errors.Is(err2, ErrNotSupported) {
		return nil, fmt.Errorf("getting map info from fdinfo: %w", err2)
	}

	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf("ObjInfo and fdinfo both failed: objinfo: %w, fdinfo: %w", err1, err2)
	}

	return mi, nil
}

// readMapInfoFromProc queries map information about the given fd from
// /proc/self/fdinfo. It only writes data into fields that have a zero value.
func readMapInfoFromProc(fd *sys.FD, mi *MapInfo) error {
	return scanFdInfo(fd, map[string]interface{}{
		"map_type":    &mi.Type,
		"map_id":      &mi.id,
		"key_size":    &mi.KeySize,
		"value_size":  &mi.ValueSize,
		"max_entries": &mi.MaxEntries,
		"map_flags":   &mi.Flags,
		"map_extra":   &mi.mapExtra,
		"memlock":     &mi.memlock,
		"frozen":      &mi.frozen,
	})
}

// ID returns the map ID.
//
// Available from 4.13.
//
// The bool return value indicates whether this optional field is available.
func (mi *MapInfo) ID() (MapID, bool) {
	return mi.id, mi.id > 0
}

// BTFID returns the BTF ID associated with the Map.
//
// The ID is only valid as long as the associated Map is kept alive.
// Available from 4.18.
//
// The bool return value indicates whether this optional field is available and
// populated. (The field may be available but not populated if the kernel
// supports the field but the Map was loaded without BTF information.)
func (mi *MapInfo) BTFID() (btf.ID, bool) {
	return mi.btf, mi.btf > 0
}

// MapExtra returns an opaque field whose meaning is map-specific.
//
// Available from 5.16.
//
// The bool return value indicates whether this optional field is available and
// populated, if it was specified during Map creation.
func (mi *MapInfo) MapExtra() (uint64, bool) {
	return mi.mapExtra, mi.mapExtra > 0
}

// Memlock returns an approximate number of bytes allocated to this map.
//
// Available from 4.10.
//
// The bool return value indicates whether this optional field is available.
func (mi *MapInfo) Memlock() (uint64, bool) {
	return mi.memlock, mi.memlock > 0
}

// Frozen indicates whether [Map.Freeze] was called on this map. If true,
// modifications from user space are not allowed.
//
// Available from 5.2. Requires access to procfs.
//
// If the kernel doesn't support map freezing, this field will always be false.
func (mi *MapInfo) Frozen() bool {
	return mi.frozen
}

// programStats holds statistics of a program.
type programStats struct {
	// Total accumulated runtime of the program ins ns.
	runtime time.Duration
	// Total number of times the program was called.
	runCount uint64
	// Total number of times the programm was NOT called.
	// Added in commit 9ed9e9ba2337 ("bpf: Count the number of times recursion was prevented").
	recursionMisses uint64
}

// programJitedInfo holds information about JITed info of a program.
type programJitedInfo struct {
	// ksyms holds the ksym addresses of the BPF program, including those of its
	// subprograms.
	//
	// Available from 4.18.
	ksyms    []uint64
	numKsyms uint32

	// insns holds the JITed machine native instructions of the program,
	// including those of its subprograms.
	//
	// Available from 4.13.
	insns    []byte
	numInsns uint32

	// lineInfos holds the JITed line infos, which are kernel addresses.
	//
	// Available from 5.0.
	lineInfos    []uint64
	numLineInfos uint32

	// lineInfoRecSize is the size of a single line info record.
	//
	// Available from 5.0.
	lineInfoRecSize uint32

	// funcLens holds the insns length of each function.
	//
	// Available from 4.18.
	funcLens    []uint32
	numFuncLens uint32
}

// ProgramInfo describes a program.
type ProgramInfo struct {
	Type ProgramType
	id   ProgramID
	// Truncated hash of the BPF bytecode. Available from 4.13.
	Tag string
	// Name as supplied by user space at load time. Available from 4.15.
	Name string

	createdByUID     uint32
	haveCreatedByUID bool
	btf              btf.ID
	stats            *programStats
	loadTime         time.Duration

	maps                 []MapID
	insns                []byte
	jitedSize            uint32
	verifiedInstructions uint32

	jitedInfo programJitedInfo

	lineInfos    []byte
	numLineInfos uint32
	funcInfos    []byte
	numFuncInfos uint32
}

func newProgramInfoFromFd(fd *sys.FD) (*ProgramInfo, error) {
	var info sys.ProgInfo
	err := sys.ObjInfo(fd, &info)
	if errors.Is(err, syscall.EINVAL) {
		return newProgramInfoFromProc(fd)
	}
	if err != nil {
		return nil, err
	}

	pi := ProgramInfo{
		Type: ProgramType(info.Type),
		id:   ProgramID(info.Id),
		Tag:  hex.EncodeToString(info.Tag[:]),
		Name: unix.ByteSliceToString(info.Name[:]),
		btf:  btf.ID(info.BtfId),
		stats: &programStats{
			runtime:         time.Duration(info.RunTimeNs),
			runCount:        info.RunCnt,
			recursionMisses: info.RecursionMisses,
		},
		jitedSize:            info.JitedProgLen,
		loadTime:             time.Duration(info.LoadTime),
		verifiedInstructions: info.VerifiedInsns,
	}

	// Start with a clean struct for the second call, otherwise we may get EFAULT.
	var info2 sys.ProgInfo

	makeSecondCall := false

	if info.NrMapIds > 0 {
		pi.maps = make([]MapID, info.NrMapIds)
		info2.NrMapIds = info.NrMapIds
		info2.MapIds = sys.NewSlicePointer(pi.maps)
		makeSecondCall = true
	} else if haveProgramInfoMapIDs() == nil {
		// This program really has no associated maps.
		pi.maps = make([]MapID, 0)
	} else {
		// The kernel doesn't report associated maps.
		pi.maps = nil
	}

	// createdByUID and NrMapIds were introduced in the same kernel version.
	if pi.maps != nil {
		pi.createdByUID = info.CreatedByUid
		pi.haveCreatedByUID = true
	}

	if info.XlatedProgLen > 0 {
		pi.insns = make([]byte, info.XlatedProgLen)
		info2.XlatedProgLen = info.XlatedProgLen
		info2.XlatedProgInsns = sys.NewSlicePointer(pi.insns)
		makeSecondCall = true
	}

	if info.NrLineInfo > 0 {
		pi.lineInfos = make([]byte, btf.LineInfoSize*info.NrLineInfo)
		info2.LineInfo = sys.NewSlicePointer(pi.lineInfos)
		info2.LineInfoRecSize = btf.LineInfoSize
		info2.NrLineInfo = info.NrLineInfo
		pi.numLineInfos = info.NrLineInfo
		makeSecondCall = true
	}

	if info.NrFuncInfo > 0 {
		pi.funcInfos = make([]byte, btf.FuncInfoSize*info.NrFuncInfo)
		info2.FuncInfo = sys.NewSlicePointer(pi.funcInfos)
		info2.FuncInfoRecSize = btf.FuncInfoSize
		info2.NrFuncInfo = info.NrFuncInfo
		pi.numFuncInfos = info.NrFuncInfo
		makeSecondCall = true
	}

	pi.jitedInfo.lineInfoRecSize = info.JitedLineInfoRecSize
	if info.JitedProgLen > 0 {
		pi.jitedInfo.numInsns = info.JitedProgLen
		pi.jitedInfo.insns = make([]byte, info.JitedProgLen)
		info2.JitedProgLen = info.JitedProgLen
		info2.JitedProgInsns = sys.NewSlicePointer(pi.jitedInfo.insns)
		makeSecondCall = true
	}

	if info.NrJitedFuncLens > 0 {
		pi.jitedInfo.numFuncLens = info.NrJitedFuncLens
		pi.jitedInfo.funcLens = make([]uint32, info.NrJitedFuncLens)
		info2.NrJitedFuncLens = info.NrJitedFuncLens
		info2.JitedFuncLens = sys.NewSlicePointer(pi.jitedInfo.funcLens)
		makeSecondCall = true
	}

	if info.NrJitedLineInfo > 0 {
		pi.jitedInfo.numLineInfos = info.NrJitedLineInfo
		pi.jitedInfo.lineInfos = make([]uint64, info.NrJitedLineInfo)
		info2.NrJitedLineInfo = info.NrJitedLineInfo
		info2.JitedLineInfo = sys.NewSlicePointer(pi.jitedInfo.lineInfos)
		info2.JitedLineInfoRecSize = info.JitedLineInfoRecSize
		makeSecondCall = true
	}

	if info.NrJitedKsyms > 0 {
		pi.jitedInfo.numKsyms = info.NrJitedKsyms
		pi.jitedInfo.ksyms = make([]uint64, info.NrJitedKsyms)
		info2.JitedKsyms = sys.NewSlicePointer(pi.jitedInfo.ksyms)
		info2.NrJitedKsyms = info.NrJitedKsyms
		makeSecondCall = true
	}

	if makeSecondCall {
		if err := sys.ObjInfo(fd, &info2); err != nil {
			return nil, err
		}
	}

	return &pi, nil
}

func newProgramInfoFromProc(fd *sys.FD) (*ProgramInfo, error) {
	var info ProgramInfo
	err := scanFdInfo(fd, map[string]interface{}{
		"prog_type": &info.Type,
		"prog_tag":  &info.Tag,
	})
	if errors.Is(err, ErrNotSupported) {
		return nil, &internal.UnsupportedFeatureError{
			Name:           "reading program info from /proc/self/fdinfo",
			MinimumVersion: internal.Version{4, 10, 0},
		}
	}
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// ID returns the program ID.
//
// Available from 4.13.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) ID() (ProgramID, bool) {
	return pi.id, pi.id > 0
}

// CreatedByUID returns the Uid that created the program.
//
// Available from 4.15.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) CreatedByUID() (uint32, bool) {
	return pi.createdByUID, pi.haveCreatedByUID
}

// BTFID returns the BTF ID associated with the program.
//
// The ID is only valid as long as the associated program is kept alive.
// Available from 5.0.
//
// The bool return value indicates whether this optional field is available and
// populated. (The field may be available but not populated if the kernel
// supports the field but the program was loaded without BTF information.)
func (pi *ProgramInfo) BTFID() (btf.ID, bool) {
	return pi.btf, pi.btf > 0
}

// RunCount returns the total number of times the program was called.
//
// Can return 0 if the collection of statistics is not enabled. See EnableStats().
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) RunCount() (uint64, bool) {
	if pi.stats != nil {
		return pi.stats.runCount, true
	}
	return 0, false
}

// Runtime returns the total accumulated runtime of the program.
//
// Can return 0 if the collection of statistics is not enabled. See EnableStats().
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) Runtime() (time.Duration, bool) {
	if pi.stats != nil {
		return pi.stats.runtime, true
	}
	return time.Duration(0), false
}

// RecursionMisses returns the total number of times the program was NOT called.
// This can happen when another bpf program is already running on the cpu, which
// is likely to happen for example when you interrupt bpf program execution.
func (pi *ProgramInfo) RecursionMisses() (uint64, bool) {
	if pi.stats != nil {
		return pi.stats.recursionMisses, true
	}
	return 0, false
}

// btfSpec returns the BTF spec associated with the program.
func (pi *ProgramInfo) btfSpec() (*btf.Spec, error) {
	id, ok := pi.BTFID()
	if !ok {
		return nil, fmt.Errorf("program created without BTF or unsupported kernel: %w", ErrNotSupported)
	}

	h, err := btf.NewHandleFromID(id)
	if err != nil {
		return nil, fmt.Errorf("get BTF handle: %w", err)
	}
	defer h.Close()

	spec, err := h.Spec(nil)
	if err != nil {
		return nil, fmt.Errorf("get BTF spec: %w", err)
	}

	return spec, nil
}

// LineInfos returns the BTF line information of the program.
//
// Available from 5.0.
//
// Requires CAP_SYS_ADMIN or equivalent for reading BTF information. Returns
// ErrNotSupported if the program was created without BTF or if the kernel
// doesn't support the field.
func (pi *ProgramInfo) LineInfos() (btf.LineOffsets, error) {
	if len(pi.lineInfos) == 0 {
		return nil, fmt.Errorf("insufficient permissions or unsupported kernel: %w", ErrNotSupported)
	}

	spec, err := pi.btfSpec()
	if err != nil {
		return nil, err
	}

	return btf.LoadLineInfos(
		bytes.NewReader(pi.lineInfos),
		internal.NativeEndian,
		pi.numLineInfos,
		spec,
	)
}

// Instructions returns the 'xlated' instruction stream of the program
// after it has been verified and rewritten by the kernel. These instructions
// cannot be loaded back into the kernel as-is, this is mainly used for
// inspecting loaded programs for troubleshooting, dumping, etc.
//
// For example, map accesses are made to reference their kernel map IDs,
// not the FDs they had when the program was inserted. Note that before
// the introduction of bpf_insn_prepare_dump in kernel 4.16, xlated
// instructions were not sanitized, making the output even less reusable
// and less likely to round-trip or evaluate to the same program Tag.
//
// The first instruction is marked as a symbol using the Program's name.
//
// If available, the instructions will be annotated with metadata from the
// BTF. This includes line information and function information. Reading
// this metadata requires CAP_SYS_ADMIN or equivalent. If capability is
// unavailable, the instructions will be returned without metadata.
//
// Available from 4.13. Requires CAP_BPF or equivalent for plain instructions.
// Requires CAP_SYS_ADMIN for instructions with metadata.
func (pi *ProgramInfo) Instructions() (asm.Instructions, error) {
	// If the calling process is not BPF-capable or if the kernel doesn't
	// support getting xlated instructions, the field will be zero.
	if len(pi.insns) == 0 {
		return nil, fmt.Errorf("insufficient permissions or unsupported kernel: %w", ErrNotSupported)
	}

	r := bytes.NewReader(pi.insns)
	var insns asm.Instructions
	if err := insns.Unmarshal(r, internal.NativeEndian); err != nil {
		return nil, fmt.Errorf("unmarshaling instructions: %w", err)
	}

	if pi.btf != 0 {
		btfh, err := btf.NewHandleFromID(pi.btf)
		if err != nil {
			// Getting a BTF handle requires CAP_SYS_ADMIN, if not available we get an -EPERM.
			// Ignore it and fall back to instructions without metadata.
			if !errors.Is(err, unix.EPERM) {
				return nil, fmt.Errorf("unable to get BTF handle: %w", err)
			}
		}

		// If we have a BTF handle, we can use it to assign metadata to the instructions.
		if btfh != nil {
			defer btfh.Close()

			spec, err := btfh.Spec(nil)
			if err != nil {
				return nil, fmt.Errorf("unable to get BTF spec: %w", err)
			}

			lineInfos, err := btf.LoadLineInfos(
				bytes.NewReader(pi.lineInfos),
				internal.NativeEndian,
				pi.numLineInfos,
				spec,
			)
			if err != nil {
				return nil, fmt.Errorf("parse line info: %w", err)
			}

			funcInfos, err := btf.LoadFuncInfos(
				bytes.NewReader(pi.funcInfos),
				internal.NativeEndian,
				pi.numFuncInfos,
				spec,
			)
			if err != nil {
				return nil, fmt.Errorf("parse func info: %w", err)
			}

			btf.AssignMetadataToInstructions(insns, funcInfos, lineInfos, btf.CORERelocationInfos{})
		}
	}

	fn := btf.FuncMetadata(&insns[0])
	name := pi.Name
	if fn != nil {
		name = fn.Name
	}
	insns[0] = insns[0].WithSymbol(name)

	return insns, nil
}

// JitedSize returns the size of the program's JIT-compiled machine code in bytes, which is the
// actual code executed on the host's CPU. This field requires the BPF JIT compiler to be enabled.
//
// Available from 4.13. Reading this metadata requires CAP_BPF or equivalent.
func (pi *ProgramInfo) JitedSize() (uint32, error) {
	if pi.jitedSize == 0 {
		return 0, fmt.Errorf("insufficient permissions, unsupported kernel, or JIT compiler disabled: %w", ErrNotSupported)
	}
	return pi.jitedSize, nil
}

// TranslatedSize returns the size of the program's translated instructions in bytes, after it has
// been verified and rewritten by the kernel.
//
// Available from 4.13. Reading this metadata requires CAP_BPF or equivalent.
func (pi *ProgramInfo) TranslatedSize() (int, error) {
	insns := len(pi.insns)
	if insns == 0 {
		return 0, fmt.Errorf("insufficient permissions or unsupported kernel: %w", ErrNotSupported)
	}
	return insns, nil
}

// MapIDs returns the maps related to the program.
//
// Available from 4.15.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) MapIDs() ([]MapID, bool) {
	return pi.maps, pi.maps != nil
}

// LoadTime returns when the program was loaded since boot time.
//
// Available from 4.15.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) LoadTime() (time.Duration, bool) {
	// loadTime and NrMapIds were introduced in the same kernel version.
	return pi.loadTime, pi.loadTime > 0
}

// VerifiedInstructions returns the number verified instructions in the program.
//
// Available from 5.16.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) VerifiedInstructions() (uint32, bool) {
	return pi.verifiedInstructions, pi.verifiedInstructions > 0
}

// JitedKsymAddrs returns the ksym addresses of the BPF program, including its
// subprograms. The addresses correspond to their symbols in /proc/kallsyms.
//
// Available from 4.18. Note that before 5.x, this field can be empty for
// programs without subprograms (bpf2bpf calls).
//
// The bool return value indicates whether this optional field is available.
//
// When a kernel address can't fit into uintptr (which is usually the case when
// running 32 bit program on a 64 bit kernel), this returns an empty slice and
// a false.
func (pi *ProgramInfo) JitedKsymAddrs() ([]uintptr, bool) {
	ksyms := make([]uintptr, 0, len(pi.jitedInfo.ksyms))
	if cap(ksyms) == 0 {
		return ksyms, false
	}
	// Check if a kernel address fits into uintptr (it might not when
	// using a 32 bit binary on a 64 bit kernel). This check should work
	// with any kernel address, since they have 1s at the highest bits.
	if a := pi.jitedInfo.ksyms[0]; uint64(uintptr(a)) != a {
		return nil, false
	}
	for _, ksym := range pi.jitedInfo.ksyms {
		ksyms = append(ksyms, uintptr(ksym))
	}
	return ksyms, true
}

// JitedInsns returns the JITed machine native instructions of the program.
//
// Available from 4.13.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) JitedInsns() ([]byte, bool) {
	return pi.jitedInfo.insns, len(pi.jitedInfo.insns) > 0
}

// JitedLineInfos returns the JITed line infos of the program.
//
// Available from 5.0.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) JitedLineInfos() ([]uint64, bool) {
	return pi.jitedInfo.lineInfos, len(pi.jitedInfo.lineInfos) > 0
}

// JitedFuncLens returns the insns length of each function in the JITed program.
//
// Available from 4.18.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) JitedFuncLens() ([]uint32, bool) {
	return pi.jitedInfo.funcLens, len(pi.jitedInfo.funcLens) > 0
}

// FuncInfos returns the offset and function information of all (sub)programs in
// a BPF program.
//
// Available from 5.0.
//
// Requires CAP_SYS_ADMIN or equivalent for reading BTF information. Returns
// ErrNotSupported if the program was created without BTF or if the kernel
// doesn't support the field.
func (pi *ProgramInfo) FuncInfos() (btf.FuncOffsets, error) {
	if len(pi.funcInfos) == 0 {
		return nil, fmt.Errorf("insufficient permissions or unsupported kernel: %w", ErrNotSupported)
	}

	spec, err := pi.btfSpec()
	if err != nil {
		return nil, err
	}

	return btf.LoadFuncInfos(
		bytes.NewReader(pi.funcInfos),
		internal.NativeEndian,
		pi.numFuncInfos,
		spec,
	)
}

func scanFdInfo(fd *sys.FD, fields map[string]interface{}) error {
	fh, err := os.Open(fmt.Sprintf("/proc/self/fdinfo/%d", fd.Int()))
	if err != nil {
		return err
	}
	defer fh.Close()

	if err := scanFdInfoReader(fh, fields); err != nil {
		return fmt.Errorf("%s: %w", fh.Name(), err)
	}
	return nil
}

func scanFdInfoReader(r io.Reader, fields map[string]interface{}) error {
	var (
		scanner = bufio.NewScanner(r)
		scanned int
	)

	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "\t", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSuffix(parts[0], ":")
		field, ok := fields[string(name)]
		if !ok {
			continue
		}

		// If field already contains a non-zero value, don't overwrite it with fdinfo.
		if zero(field) {
			if n, err := fmt.Sscanln(parts[1], field); err != nil || n != 1 {
				return fmt.Errorf("can't parse field %s: %v", name, err)
			}
		}

		scanned++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning fdinfo: %w", err)
	}

	if len(fields) > 0 && scanned == 0 {
		return ErrNotSupported
	}

	return nil
}

func zero(arg any) bool {
	v := reflect.ValueOf(arg)

	// Unwrap pointers and interfaces.
	for v.Kind() == reflect.Pointer ||
		v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	return v.IsZero()
}

// EnableStats starts the measuring of the runtime
// and run counts of eBPF programs.
//
// Collecting statistics can have an impact on the performance.
//
// Requires at least 5.8.
func EnableStats(which uint32) (io.Closer, error) {
	fd, err := sys.EnableStats(&sys.EnableStatsAttr{
		Type: which,
	})
	if err != nil {
		return nil, err
	}
	return fd, nil
}

var haveProgramInfoMapIDs = internal.NewFeatureTest("map IDs in program info", func() error {
	prog, err := progLoad(asm.Instructions{
		asm.LoadImm(asm.R0, 0, asm.DWord),
		asm.Return(),
	}, SocketFilter, "MIT")
	if err != nil {
		return err
	}
	defer prog.Close()

	err = sys.ObjInfo(prog, &sys.ProgInfo{
		// NB: Don't need to allocate MapIds since the program isn't using
		// any maps.
		NrMapIds: 1,
	})
	if errors.Is(err, unix.EINVAL) {
		// Most likely the syscall doesn't exist.
		return internal.ErrNotSupported
	}
	if errors.Is(err, unix.E2BIG) {
		// We've hit check_uarg_tail_zero on older kernels.
		return internal.ErrNotSupported
	}

	return err
}, "4.15")
