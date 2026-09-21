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
	"time"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/platform"
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

// minimalMapInfoFromFd queries the minimum information needed to create a Map
// based on a file descriptor. This requires the map type, key/value sizes,
// maxentries and flags.
//
// Does not fall back to fdinfo since the version gap between fdinfo (4.10) and
// [sys.ObjInfo] (4.13) is small and both kernels are EOL since at least Nov
// 2017.
//
// Requires at least Linux 4.13.
func minimalMapInfoFromFd(fd *sys.FD) (*MapInfo, error) {
	var info sys.MapInfo
	if err := sys.ObjInfo(fd, &info); err != nil {
		return nil, fmt.Errorf("getting object info: %w", err)
	}

	typ, err := MapTypeForPlatform(platform.Native, info.Type)
	if err != nil {
		return nil, fmt.Errorf("map type: %w", err)
	}

	return &MapInfo{
		Type:       typ,
		KeySize:    info.KeySize,
		ValueSize:  info.ValueSize,
		MaxEntries: info.MaxEntries,
		Flags:      uint32(info.MapFlags),
		Name:       unix.ByteSliceToString(info.Name[:]),
	}, nil
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

	typ, err := MapTypeForPlatform(platform.Native, info.Type)
	if err != nil {
		return nil, fmt.Errorf("map type: %w", err)
	}

	mi := &MapInfo{
		typ,
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
	var mapType uint32
	err := scanFdInfo(fd, map[string]any{
		"map_type":    &mapType,
		"map_id":      &mi.id,
		"key_size":    &mi.KeySize,
		"value_size":  &mi.ValueSize,
		"max_entries": &mi.MaxEntries,
		"map_flags":   &mi.Flags,
		"map_extra":   &mi.mapExtra,
		"memlock":     &mi.memlock,
		"frozen":      &mi.frozen,
	})
	if err != nil {
		return err
	}

	if mi.Type == 0 {
		mi.Type, err = MapTypeForPlatform(platform.Linux, mapType)
		if err != nil {
			return fmt.Errorf("map type: %w", err)
		}
	}

	return nil
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

// ProgramStats contains runtime statistics for a single [Program], returned by
// [Program.Stats].
//
// Will contain mostly zero values if the collection of statistics is not
// enabled, see [EnableStats].
type ProgramStats struct {
	// Total accumulated runtime of the Program.
	//
	// Requires at least Linux 5.8.
	Runtime time.Duration

	// Total number of times the Program has executed.
	//
	// Requires at least Linux 5.8.
	RunCount uint64

	// Total number of times the program was not executed due to recursion. This
	// can happen when another bpf program is already running on the cpu, when bpf
	// program execution is interrupted, for example.
	//
	// Requires at least Linux 5.12.
	RecursionMisses uint64
}

func newProgramStatsFromFd(fd *sys.FD) (*ProgramStats, error) {
	var info sys.ProgInfo
	if err := sys.ObjInfo(fd, &info); err != nil {
		return nil, fmt.Errorf("getting program info: %w", err)
	}

	return &ProgramStats{
		Runtime:         time.Duration(info.RunTimeNs),
		RunCount:        info.RunCnt,
		RecursionMisses: info.RecursionMisses,
	}, nil
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

// ProgramInfo describes a Program's immutable metadata. For runtime statistics,
// see [ProgramStats].
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
	loadTime         time.Duration

	restricted bool

	maps                 []MapID
	insns                []byte
	numInsns             uint32
	jitedSize            uint32
	verifiedInstructions uint32

	jitedInfo programJitedInfo

	lineInfos    []byte
	numLineInfos uint32
	funcInfos    []byte
	numFuncInfos uint32

	memlock uint64
}

// minimalProgramFromFd queries the minimum information needed to create a
// Program based on a file descriptor, requiring at least the program type.
//
// Does not fall back to fdinfo since the version gap between fdinfo (4.10) and
// [sys.ObjInfo] (4.13) is small and both kernels are EOL since at least Nov
// 2017.
//
// Requires at least Linux 4.13.
func minimalProgramInfoFromFd(fd *sys.FD) (*ProgramInfo, error) {
	var info sys.ProgInfo
	if err := sys.ObjInfo(fd, &info); err != nil {
		return nil, fmt.Errorf("getting object info: %w", err)
	}

	typ, err := ProgramTypeForPlatform(platform.Native, info.Type)
	if err != nil {
		return nil, fmt.Errorf("program type: %w", err)
	}

	return &ProgramInfo{
		Type: typ,
		Name: unix.ByteSliceToString(info.Name[:]),
	}, nil
}

// newProgramInfoFromFd queries program information about the given fd.
//
// [sys.ObjInfo] is attempted first, supplementing any missing values with
// information from /proc/self/fdinfo. Ignores EINVAL from ObjInfo as well as
// ErrNotSupported from reading fdinfo (indicating the file exists, but no
// fields of interest were found). If both fail, an error is always returned.
func newProgramInfoFromFd(fd *sys.FD) (*ProgramInfo, error) {
	var info sys.ProgInfo
	err1 := sys.ObjInfo(fd, &info)
	// EINVAL means the kernel doesn't support BPF_OBJ_GET_INFO_BY_FD. Continue
	// with fdinfo if that's the case.
	if err1 != nil && !errors.Is(err1, unix.EINVAL) {
		return nil, fmt.Errorf("getting object info: %w", err1)
	}

	typ, err := ProgramTypeForPlatform(platform.Native, info.Type)
	if err != nil {
		return nil, fmt.Errorf("program type: %w", err)
	}

	pi := ProgramInfo{
		Type:                 typ,
		id:                   ProgramID(info.Id),
		Tag:                  hex.EncodeToString(info.Tag[:]),
		Name:                 unix.ByteSliceToString(info.Name[:]),
		btf:                  btf.ID(info.BtfId),
		jitedSize:            info.JitedProgLen,
		loadTime:             time.Duration(info.LoadTime),
		verifiedInstructions: info.VerifiedInsns,
		numInsns:             info.XlatedProgLen,
	}

	// Supplement OBJ_INFO with data from /proc/self/fdinfo. It contains fields
	// like memlock that is not present in OBJ_INFO.
	err2 := readProgramInfoFromProc(fd, &pi)
	if err2 != nil && !errors.Is(err2, ErrNotSupported) {
		return nil, fmt.Errorf("getting map info from fdinfo: %w", err2)
	}

	if err1 != nil && err2 != nil {
		return nil, fmt.Errorf("ObjInfo and fdinfo both failed: objinfo: %w, fdinfo: %w", err1, err2)
	}

	if platform.IsWindows && info.Tag == [8]uint8{} {
		// Windows doesn't support the tag field, clear it for now.
		pi.Tag = ""
	}

	// Start with a clean struct for the second call, otherwise we may get EFAULT.
	var info2 sys.ProgInfo

	makeSecondCall := false

	if info.NrMapIds > 0 {
		pi.maps = make([]MapID, info.NrMapIds)
		info2.NrMapIds = info.NrMapIds
		info2.MapIds = sys.SlicePointer(pi.maps)
		makeSecondCall = true
	} else if haveProgramInfoMapIDs() == nil {
		// This program really has no associated maps.
		pi.maps = make([]MapID, 0)
	} else {
		// The kernel doesn't report associated maps.
		pi.maps = nil
	}

	// createdByUID and NrMapIds were introduced in the same kernel version.
	if pi.maps != nil && platform.IsLinux {
		pi.createdByUID = info.CreatedByUid
		pi.haveCreatedByUID = true
	}

	if info.XlatedProgLen > 0 {
		pi.insns = make([]byte, info.XlatedProgLen)
		var info3 sys.ProgInfo
		info3.XlatedProgLen = info.XlatedProgLen
		info3.XlatedProgInsns = sys.SlicePointer(pi.insns)

		// When kernel.kptr_restrict and net.core.bpf_jit_harden are both set, it causes the
		// syscall to abort when trying to readback xlated instructions, skipping other info
		// as well. So request xlated instructions separately.
		if err := sys.ObjInfo(fd, &info3); err != nil {
			return nil, err
		}
		if info3.XlatedProgInsns.IsNil() {
			pi.restricted = true
			pi.insns = nil
		}
	}

	if info.NrLineInfo > 0 {
		pi.lineInfos = make([]byte, btf.LineInfoSize*info.NrLineInfo)
		info2.LineInfo = sys.SlicePointer(pi.lineInfos)
		info2.LineInfoRecSize = btf.LineInfoSize
		info2.NrLineInfo = info.NrLineInfo
		pi.numLineInfos = info.NrLineInfo
		makeSecondCall = true
	}

	if info.NrFuncInfo > 0 {
		pi.funcInfos = make([]byte, btf.FuncInfoSize*info.NrFuncInfo)
		info2.FuncInfo = sys.SlicePointer(pi.funcInfos)
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
		info2.JitedProgInsns = sys.SlicePointer(pi.jitedInfo.insns)
		makeSecondCall = true
	}

	if info.NrJitedFuncLens > 0 {
		pi.jitedInfo.numFuncLens = info.NrJitedFuncLens
		pi.jitedInfo.funcLens = make([]uint32, info.NrJitedFuncLens)
		info2.NrJitedFuncLens = info.NrJitedFuncLens
		info2.JitedFuncLens = sys.SlicePointer(pi.jitedInfo.funcLens)
		makeSecondCall = true
	}

	if info.NrJitedLineInfo > 0 {
		pi.jitedInfo.numLineInfos = info.NrJitedLineInfo
		pi.jitedInfo.lineInfos = make([]uint64, info.NrJitedLineInfo)
		info2.NrJitedLineInfo = info.NrJitedLineInfo
		info2.JitedLineInfo = sys.SlicePointer(pi.jitedInfo.lineInfos)
		info2.JitedLineInfoRecSize = info.JitedLineInfoRecSize
		makeSecondCall = true
	}

	if info.NrJitedKsyms > 0 {
		pi.jitedInfo.numKsyms = info.NrJitedKsyms
		pi.jitedInfo.ksyms = make([]uint64, info.NrJitedKsyms)
		info2.JitedKsyms = sys.SlicePointer(pi.jitedInfo.ksyms)
		info2.NrJitedKsyms = info.NrJitedKsyms
		makeSecondCall = true
	}

	if makeSecondCall {
		if err := sys.ObjInfo(fd, &info2); err != nil {
			return nil, err
		}
		if info.JitedProgLen > 0 && info2.JitedProgInsns.IsNil() {
			// JIT information is not available due to kernel.kptr_restrict
			pi.jitedInfo.lineInfos = nil
			pi.jitedInfo.ksyms = nil
			pi.jitedInfo.insns = nil
			pi.jitedInfo.funcLens = nil
		}
	}

	if len(pi.Name) == len(info.Name)-1 { // Possibly truncated, check BTF info for full name
		name, err := readNameFromFunc(&pi)
		if err == nil {
			pi.Name = name
		} // If an error occurs, keep the truncated name, which is better than none
	}

	return &pi, nil
}

func readNameFromFunc(pi *ProgramInfo) (string, error) {
	if pi.numFuncInfos == 0 {
		return "", errors.New("no function info")
	}

	spec, err := pi.btfSpec()
	if err != nil {
		return "", err
	}

	funcInfos, err := btf.LoadFuncInfos(
		bytes.NewReader(pi.funcInfos),
		internal.NativeEndian,
		pi.numFuncInfos,
		spec,
	)
	if err != nil {
		return "", err
	}

	for _, funcInfo := range funcInfos {
		if funcInfo.Offset == 0 { // Information about the whole program
			return funcInfo.Func.Name, nil
		}
	}
	return "", errors.New("no function info about program")
}

func readProgramInfoFromProc(fd *sys.FD, pi *ProgramInfo) error {
	var progType uint32
	err := scanFdInfo(fd, map[string]any{
		"prog_type": &progType,
		"prog_tag":  &pi.Tag,
		"memlock":   &pi.memlock,
	})
	if errors.Is(err, ErrNotSupported) && !errors.Is(err, internal.ErrNotSupportedOnOS) {
		return &internal.UnsupportedFeatureError{
			Name:           "reading program info from /proc/self/fdinfo",
			MinimumVersion: internal.Version{4, 10, 0},
		}
	}
	if err != nil {
		return err
	}

	pi.Type, err = ProgramTypeForPlatform(platform.Linux, progType)
	if err != nil {
		return fmt.Errorf("program type: %w", err)
	}

	return nil
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

// ErrRestrictedKernel is returned when kernel address information is restricted
// by kernel.kptr_restrict and/or net.core.bpf_jit_harden sysctls.
var ErrRestrictedKernel = internal.ErrRestrictedKernel

// LineInfos returns the BTF line information of the program.
//
// Available from 5.0.
//
// Returns an error wrapping [ErrRestrictedKernel] if line infos are restricted
// by sysctls.
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
// Returns an error wrapping [ErrRestrictedKernel] if instructions are
// restricted by sysctls.
//
// Available from 4.13. Requires CAP_BPF or equivalent for plain instructions.
// Requires CAP_SYS_ADMIN for instructions with metadata.
func (pi *ProgramInfo) Instructions() (asm.Instructions, error) {
	if platform.IsWindows && len(pi.insns) == 0 {
		return nil, fmt.Errorf("read instructions: %w", internal.ErrNotSupportedOnOS)
	}

	if pi.restricted {
		return nil, fmt.Errorf("instructions: %w", ErrRestrictedKernel)
	}

	// If the calling process is not BPF-capable or if the kernel doesn't
	// support getting xlated instructions, the field will be zero.
	if len(pi.insns) == 0 {
		return nil, fmt.Errorf("insufficient permissions or unsupported kernel: %w", ErrNotSupported)
	}

	r := bytes.NewReader(pi.insns)
	insns, err := asm.AppendInstructions(nil, r, internal.NativeEndian, platform.Native)
	if err != nil {
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

			lineInfos, err := btf.LoadLineInfos(bytes.NewReader(pi.lineInfos), internal.NativeEndian, pi.numLineInfos, spec)
			if err != nil {
				return nil, fmt.Errorf("parse line info: %w", err)
			}

			funcInfos, err := btf.LoadFuncInfos(bytes.NewReader(pi.funcInfos), internal.NativeEndian, pi.numFuncInfos, spec)
			if err != nil {
				return nil, fmt.Errorf("parse func info: %w", err)
			}

			iter := insns.Iterate()
			for iter.Next() {
				assignMetadata(iter.Ins, iter.Offset, &funcInfos, &lineInfos, nil)
			}
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

// JitedSize returns the size of the program's JIT-compiled machine code in
// bytes, which is the actual code executed on the host's CPU. This field
// requires the BPF JIT compiler to be enabled.
//
// Returns an error wrapping [ErrRestrictedKernel] if jited program size is
// restricted by sysctls.
//
// Available from 4.13. Reading this metadata requires CAP_BPF or equivalent.
func (pi *ProgramInfo) JitedSize() (uint32, error) {
	if pi.jitedSize == 0 {
		return 0, fmt.Errorf("insufficient permissions, unsupported kernel, or JIT compiler disabled: %w", ErrNotSupported)
	}
	return pi.jitedSize, nil
}

// TranslatedSize returns the size of the program's translated instructions in
// bytes, after it has been verified and rewritten by the kernel.
//
// Available from 4.13. Reading this metadata requires CAP_BPF or equivalent.
func (pi *ProgramInfo) TranslatedSize() (int, error) {
	if pi.numInsns == 0 {
		return 0, fmt.Errorf("insufficient permissions or unsupported kernel: %w", ErrNotSupported)
	}
	return int(pi.numInsns), nil
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
// Returns an error wrapping [ErrRestrictedKernel] if function information is
// restricted by sysctls.
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

// ProgramInfo returns an approximate number of bytes allocated to this program.
//
// Available from 4.10.
//
// The bool return value indicates whether this optional field is available.
func (pi *ProgramInfo) Memlock() (uint64, bool) {
	return pi.memlock, pi.memlock > 0
}

func scanFdInfo(fd *sys.FD, fields map[string]any) error {
	if platform.IsWindows {
		return fmt.Errorf("read fdinfo: %w", internal.ErrNotSupportedOnOS)
	}

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

func scanFdInfoReader(r io.Reader, fields map[string]any) error {
	var (
		scanner = bufio.NewScanner(r)
		scanned int
		reader  bytes.Reader
	)

	for scanner.Scan() {
		key, rest, found := bytes.Cut(scanner.Bytes(), []byte(":"))
		if !found {
			// Line doesn't contain a colon, skip.
			continue
		}
		field, ok := fields[string(key)]
		if !ok {
			continue
		}
		// If field already contains a non-zero value, don't overwrite it with fdinfo.
		if !zero(field) {
			scanned++
			continue
		}

		// Cut the \t following the : as well as any potential trailing whitespace.
		rest = bytes.TrimSpace(rest)

		reader.Reset(rest)
		if n, err := fmt.Fscan(&reader, field); err != nil || n != 1 {
			return fmt.Errorf("can't parse field %s: %v", key, err)
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

// EnableStats starts collecting runtime statistics of eBPF programs, like the
// amount of program executions and the cumulative runtime.
//
// Specify a BPF_STATS_* constant to select which statistics to collect, like
// [unix.BPF_STATS_RUN_TIME]. Closing the returned [io.Closer] will stop
// collecting statistics.
//
// Collecting statistics may have a performance impact.
//
// Requires at least Linux 5.8.
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
	if platform.IsWindows {
		// We only support efW versions which have this feature, no need to probe.
		return nil
	}

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
}, "4.15", "windows:0.21.0")
