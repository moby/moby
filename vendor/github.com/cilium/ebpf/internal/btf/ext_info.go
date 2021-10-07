package btf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
)

// extInfo contains extended program metadata.
//
// It is indexed per section.
type extInfo struct {
	funcInfos map[string]FuncInfos
	lineInfos map[string]LineInfos
	relos     map[string]CoreRelos
}

// loadExtInfos parses the .BTF.ext section into its constituent parts.
func loadExtInfos(r io.ReaderAt, bo binary.ByteOrder, strings stringTable) (*extInfo, error) {
	// Open unbuffered section reader. binary.Read() calls io.ReadFull on
	// the header structs, resulting in one syscall per header.
	headerRd := io.NewSectionReader(r, 0, math.MaxInt64)
	extHeader, err := parseBTFExtHeader(headerRd, bo)
	if err != nil {
		return nil, fmt.Errorf("parsing BTF extension header: %w", err)
	}

	coreHeader, err := parseBTFExtCoreHeader(headerRd, bo, extHeader)
	if err != nil {
		return nil, fmt.Errorf("parsing BTF CO-RE header: %w", err)
	}

	buf := internal.NewBufferedSectionReader(r, extHeader.funcInfoStart(), int64(extHeader.FuncInfoLen))
	funcInfos, err := parseFuncInfos(buf, bo, strings)
	if err != nil {
		return nil, fmt.Errorf("parsing BTF function info: %w", err)
	}

	buf = internal.NewBufferedSectionReader(r, extHeader.lineInfoStart(), int64(extHeader.LineInfoLen))
	lineInfos, err := parseLineInfos(buf, bo, strings)
	if err != nil {
		return nil, fmt.Errorf("parsing BTF line info: %w", err)
	}

	relos := make(map[string]CoreRelos)
	if coreHeader != nil && coreHeader.CoreReloOff > 0 && coreHeader.CoreReloLen > 0 {
		buf = internal.NewBufferedSectionReader(r, extHeader.coreReloStart(coreHeader), int64(coreHeader.CoreReloLen))
		relos, err = parseCoreRelos(buf, bo, strings)
		if err != nil {
			return nil, fmt.Errorf("parsing CO-RE relocation info: %w", err)
		}
	}

	return &extInfo{funcInfos, lineInfos, relos}, nil
}

// btfExtHeader is found at the start of the .BTF.ext section.
type btfExtHeader struct {
	Magic   uint16
	Version uint8
	Flags   uint8

	// HdrLen is larger than the size of struct btfExtHeader when it is
	// immediately followed by a btfExtCoreHeader.
	HdrLen uint32

	FuncInfoOff uint32
	FuncInfoLen uint32
	LineInfoOff uint32
	LineInfoLen uint32
}

// parseBTFExtHeader parses the header of the .BTF.ext section.
func parseBTFExtHeader(r io.Reader, bo binary.ByteOrder) (*btfExtHeader, error) {
	var header btfExtHeader
	if err := binary.Read(r, bo, &header); err != nil {
		return nil, fmt.Errorf("can't read header: %v", err)
	}

	if header.Magic != btfMagic {
		return nil, fmt.Errorf("incorrect magic value %v", header.Magic)
	}

	if header.Version != 1 {
		return nil, fmt.Errorf("unexpected version %v", header.Version)
	}

	if header.Flags != 0 {
		return nil, fmt.Errorf("unsupported flags %v", header.Flags)
	}

	if int64(header.HdrLen) < int64(binary.Size(&header)) {
		return nil, fmt.Errorf("header length shorter than btfExtHeader size")
	}

	return &header, nil
}

// funcInfoStart returns the offset from the beginning of the .BTF.ext section
// to the start of its func_info entries.
func (h *btfExtHeader) funcInfoStart() int64 {
	return int64(h.HdrLen + h.FuncInfoOff)
}

// lineInfoStart returns the offset from the beginning of the .BTF.ext section
// to the start of its line_info entries.
func (h *btfExtHeader) lineInfoStart() int64 {
	return int64(h.HdrLen + h.LineInfoOff)
}

// coreReloStart returns the offset from the beginning of the .BTF.ext section
// to the start of its CO-RE relocation entries.
func (h *btfExtHeader) coreReloStart(ch *btfExtCoreHeader) int64 {
	return int64(h.HdrLen + ch.CoreReloOff)
}

// btfExtCoreHeader is found right after the btfExtHeader when its HdrLen
// field is larger than its size.
type btfExtCoreHeader struct {
	CoreReloOff uint32
	CoreReloLen uint32
}

// parseBTFExtCoreHeader parses the tail of the .BTF.ext header. If additional
// header bytes are present, extHeader.HdrLen will be larger than the struct,
// indicating the presence of a CO-RE extension header.
func parseBTFExtCoreHeader(r io.Reader, bo binary.ByteOrder, extHeader *btfExtHeader) (*btfExtCoreHeader, error) {
	extHdrSize := int64(binary.Size(&extHeader))
	remainder := int64(extHeader.HdrLen) - extHdrSize

	if remainder == 0 {
		return nil, nil
	}

	var coreHeader btfExtCoreHeader
	if err := binary.Read(r, bo, &coreHeader); err != nil {
		return nil, fmt.Errorf("can't read header: %v", err)
	}

	return &coreHeader, nil
}

type btfExtInfoSec struct {
	SecNameOff uint32
	NumInfo    uint32
}

// parseExtInfoSec parses a btf_ext_info_sec header within .BTF.ext,
// appearing within func_info and line_info sub-sections.
// These headers appear once for each program section in the ELF and are
// followed by one or more func/line_info records for the section.
func parseExtInfoSec(r io.Reader, bo binary.ByteOrder, strings stringTable) (string, *btfExtInfoSec, error) {
	var infoHeader btfExtInfoSec
	if err := binary.Read(r, bo, &infoHeader); err != nil {
		return "", nil, fmt.Errorf("read ext info header: %w", err)
	}

	secName, err := strings.Lookup(infoHeader.SecNameOff)
	if err != nil {
		return "", nil, fmt.Errorf("get section name: %w", err)
	}
	if secName == "" {
		return "", nil, fmt.Errorf("extinfo header refers to empty section name")
	}

	if infoHeader.NumInfo == 0 {
		return "", nil, fmt.Errorf("section %s has zero records", secName)
	}

	return secName, &infoHeader, nil
}

// parseExtInfoRecordSize parses the uint32 at the beginning of a func_infos
// or line_infos segment that describes the length of all extInfoRecords in
// that segment.
func parseExtInfoRecordSize(r io.Reader, bo binary.ByteOrder) (uint32, error) {
	const maxRecordSize = 256

	var recordSize uint32
	if err := binary.Read(r, bo, &recordSize); err != nil {
		return 0, fmt.Errorf("can't read record size: %v", err)
	}

	if recordSize < 4 {
		// Need at least InsnOff worth of bytes per record.
		return 0, errors.New("record size too short")
	}
	if recordSize > maxRecordSize {
		return 0, fmt.Errorf("record size %v exceeds %v", recordSize, maxRecordSize)
	}

	return recordSize, nil
}

// FuncInfo represents the location and type ID of a function in a BPF ELF.
type FuncInfo struct {
	// Instruction offset of the function within an ELF section.
	// Always zero after parsing a funcinfo from an ELF, instruction streams
	// are split on function boundaries.
	InsnOff uint32
	TypeID  TypeID
}

// Name looks up the FuncInfo's corresponding function name in the given spec.
func (fi FuncInfo) Name(spec *Spec) (string, error) {
	// Look up function name based on type ID.
	typ, err := spec.TypeByID(fi.TypeID)
	if err != nil {
		return "", fmt.Errorf("looking up type by ID: %w", err)
	}
	if _, ok := typ.(*Func); !ok {
		return "", fmt.Errorf("type ID %d is a %T, but expected a Func", fi.TypeID, typ)
	}

	// C doesn't have anonymous functions, but check just in case.
	if name := typ.TypeName(); name != "" {
		return name, nil
	}

	return "", fmt.Errorf("Func with type ID %d doesn't have a name", fi.TypeID)
}

// Marshal writes the binary representation of the FuncInfo to w.
// The function offset is converted from bytes to instructions.
func (fi FuncInfo) Marshal(w io.Writer, offset uint64) error {
	fi.InsnOff += uint32(offset)
	// The kernel expects offsets in number of raw bpf instructions,
	// while the ELF tracks it in bytes.
	fi.InsnOff /= asm.InstructionSize
	return binary.Write(w, internal.NativeEndian, fi)
}

type FuncInfos []FuncInfo

// funcForOffset returns the function that the instruction at the given
// ELF section offset belongs to.
//
// For example, consider an ELF section that contains 3 functions (a, b, c)
// at offsets 0, 10 and 15 respectively. Offset 5 will return function a,
// offset 12 will return b, offset >= 15 will return c, etc.
func (infos FuncInfos) funcForOffset(offset uint32) *FuncInfo {
	for n, fi := range infos {
		// Iterator went past the offset the caller is looking for,
		// no point in continuing the search.
		if offset < fi.InsnOff {
			return nil
		}

		// If there is no next item in the list, or if the given offset
		// is smaller than the next function, the offset belongs to
		// the current function.
		if n+1 >= len(infos) || offset < infos[n+1].InsnOff {
			return &fi
		}
	}

	return nil
}

// parseLineInfos parses a func_info sub-section within .BTF.ext ito a map of
// func infos indexed by section name.
func parseFuncInfos(r io.Reader, bo binary.ByteOrder, strings stringTable) (map[string]FuncInfos, error) {
	recordSize, err := parseExtInfoRecordSize(r, bo)
	if err != nil {
		return nil, err
	}

	result := make(map[string]FuncInfos)
	for {
		secName, infoHeader, err := parseExtInfoSec(r, bo, strings)
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}

		records, err := parseFuncInfoRecords(r, bo, recordSize, infoHeader.NumInfo)
		if err != nil {
			return nil, fmt.Errorf("section %v: %w", secName, err)
		}

		result[secName] = records
	}
}

// parseFuncInfoRecords parses a stream of func_infos into a funcInfos.
// These records appear after a btf_ext_info_sec header in the func_info
// sub-section of .BTF.ext.
func parseFuncInfoRecords(r io.Reader, bo binary.ByteOrder, recordSize uint32, recordNum uint32) (FuncInfos, error) {
	var out FuncInfos
	var fi FuncInfo

	if exp, got := uint32(binary.Size(fi)), recordSize; exp != got {
		// BTF blob's record size is longer than we know how to parse.
		return nil, fmt.Errorf("expected FuncInfo record size %d, but BTF blob contains %d", exp, got)
	}

	for i := uint32(0); i < recordNum; i++ {
		if err := binary.Read(r, bo, &fi); err != nil {
			return nil, fmt.Errorf("can't read function info: %v", err)
		}

		if fi.InsnOff%asm.InstructionSize != 0 {
			return nil, fmt.Errorf("offset %v is not aligned with instruction size", fi.InsnOff)
		}

		out = append(out, fi)
	}

	return out, nil
}

// LineInfo represents the location and contents of a single line of source
// code a BPF ELF was compiled from.
type LineInfo struct {
	// Instruction offset of the function within an ELF section.
	// After parsing a LineInfo from an ELF, this offset is relative to
	// the function body instead of an ELF section.
	InsnOff     uint32
	FileNameOff uint32
	LineOff     uint32
	LineCol     uint32
}

// Marshal writes the binary representation of the LineInfo to w.
// The instruction offset is converted from bytes to instructions.
func (li LineInfo) Marshal(w io.Writer, offset uint64) error {
	li.InsnOff += uint32(offset)
	// The kernel expects offsets in number of raw bpf instructions,
	// while the ELF tracks it in bytes.
	li.InsnOff /= asm.InstructionSize
	return binary.Write(w, internal.NativeEndian, li)
}

type LineInfos []LineInfo

// Marshal writes the binary representation of the LineInfos to w.
func (li LineInfos) Marshal(w io.Writer, off uint64) error {
	if len(li) == 0 {
		return nil
	}

	for _, info := range li {
		if err := info.Marshal(w, off); err != nil {
			return err
		}
	}

	return nil
}

// parseLineInfos parses a line_info sub-section within .BTF.ext ito a map of
// line infos indexed by section name.
func parseLineInfos(r io.Reader, bo binary.ByteOrder, strings stringTable) (map[string]LineInfos, error) {
	recordSize, err := parseExtInfoRecordSize(r, bo)
	if err != nil {
		return nil, err
	}

	result := make(map[string]LineInfos)
	for {
		secName, infoHeader, err := parseExtInfoSec(r, bo, strings)
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}

		records, err := parseLineInfoRecords(r, bo, recordSize, infoHeader.NumInfo)
		if err != nil {
			return nil, fmt.Errorf("section %v: %w", secName, err)
		}

		result[secName] = records
	}
}

// parseLineInfoRecords parses a stream of line_infos into a lineInfos.
// These records appear after a btf_ext_info_sec header in the line_info
// sub-section of .BTF.ext.
func parseLineInfoRecords(r io.Reader, bo binary.ByteOrder, recordSize uint32, recordNum uint32) (LineInfos, error) {
	var out LineInfos
	var li LineInfo

	if exp, got := uint32(binary.Size(li)), recordSize; exp != got {
		// BTF blob's record size is longer than we know how to parse.
		return nil, fmt.Errorf("expected LineInfo record size %d, but BTF blob contains %d", exp, got)
	}

	for i := uint32(0); i < recordNum; i++ {
		if err := binary.Read(r, bo, &li); err != nil {
			return nil, fmt.Errorf("can't read line info: %v", err)
		}

		if li.InsnOff%asm.InstructionSize != 0 {
			return nil, fmt.Errorf("offset %v is not aligned with instruction size", li.InsnOff)
		}

		out = append(out, li)
	}

	return out, nil
}

// bpfCoreRelo matches the kernel's struct bpf_core_relo.
type bpfCoreRelo struct {
	InsnOff      uint32
	TypeID       TypeID
	AccessStrOff uint32
	Kind         COREKind
}

type CoreRelo struct {
	insnOff  uint32
	typeID   TypeID
	accessor coreAccessor
	kind     COREKind
}

type CoreRelos []CoreRelo

var extInfoReloSize = binary.Size(bpfCoreRelo{})

// parseCoreRelos parses a core_relos sub-section within .BTF.ext ito a map of
// CO-RE relocations indexed by section name.
func parseCoreRelos(r io.Reader, bo binary.ByteOrder, strings stringTable) (map[string]CoreRelos, error) {
	recordSize, err := parseExtInfoRecordSize(r, bo)
	if err != nil {
		return nil, err
	}

	if recordSize != uint32(extInfoReloSize) {
		return nil, fmt.Errorf("expected record size %d, got %d", extInfoReloSize, recordSize)
	}

	result := make(map[string]CoreRelos)
	for {
		secName, infoHeader, err := parseExtInfoSec(r, bo, strings)
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}

		records, err := parseCoreReloRecords(r, bo, recordSize, infoHeader.NumInfo, strings)
		if err != nil {
			return nil, fmt.Errorf("section %v: %w", secName, err)
		}

		result[secName] = records
	}
}

// parseCoreReloRecords parses a stream of CO-RE relocation entries into a
// coreRelos. These records appear after a btf_ext_info_sec header in the
// core_relos sub-section of .BTF.ext.
func parseCoreReloRecords(r io.Reader, bo binary.ByteOrder, recordSize uint32, recordNum uint32, strings stringTable) (CoreRelos, error) {
	var out CoreRelos

	var relo bpfCoreRelo
	for i := uint32(0); i < recordNum; i++ {
		if err := binary.Read(r, bo, &relo); err != nil {
			return nil, fmt.Errorf("can't read CO-RE relocation: %v", err)
		}

		if relo.InsnOff%asm.InstructionSize != 0 {
			return nil, fmt.Errorf("offset %v is not aligned with instruction size", relo.InsnOff)
		}

		accessorStr, err := strings.Lookup(relo.AccessStrOff)
		if err != nil {
			return nil, err
		}

		accessor, err := parseCoreAccessor(accessorStr)
		if err != nil {
			return nil, fmt.Errorf("accessor %q: %s", accessorStr, err)
		}

		out = append(out, CoreRelo{
			relo.InsnOff,
			relo.TypeID,
			accessor,
			relo.Kind,
		})
	}

	return out, nil
}
