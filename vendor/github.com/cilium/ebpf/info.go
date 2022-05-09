package ebpf

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/btf"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// MapInfo describes a map.
type MapInfo struct {
	Type       MapType
	id         MapID
	KeySize    uint32
	ValueSize  uint32
	MaxEntries uint32
	Flags      uint32
	// Name as supplied by user space at load time. Available from 4.15.
	Name string
}

func newMapInfoFromFd(fd *sys.FD) (*MapInfo, error) {
	var info sys.MapInfo
	err := sys.ObjInfo(fd, &info)
	if errors.Is(err, syscall.EINVAL) {
		return newMapInfoFromProc(fd)
	}
	if err != nil {
		return nil, err
	}

	return &MapInfo{
		MapType(info.Type),
		MapID(info.Id),
		info.KeySize,
		info.ValueSize,
		info.MaxEntries,
		info.MapFlags,
		unix.ByteSliceToString(info.Name[:]),
	}, nil
}

func newMapInfoFromProc(fd *sys.FD) (*MapInfo, error) {
	var mi MapInfo
	err := scanFdInfo(fd, map[string]interface{}{
		"map_type":    &mi.Type,
		"key_size":    &mi.KeySize,
		"value_size":  &mi.ValueSize,
		"max_entries": &mi.MaxEntries,
		"map_flags":   &mi.Flags,
	})
	if err != nil {
		return nil, err
	}
	return &mi, nil
}

// ID returns the map ID.
//
// Available from 4.13.
//
// The bool return value indicates whether this optional field is available.
func (mi *MapInfo) ID() (MapID, bool) {
	return mi.id, mi.id > 0
}

// programStats holds statistics of a program.
type programStats struct {
	// Total accumulated runtime of the program ins ns.
	runtime time.Duration
	// Total number of times the program was called.
	runCount uint64
}

// ProgramInfo describes a program.
type ProgramInfo struct {
	Type ProgramType
	id   ProgramID
	// Truncated hash of the BPF bytecode. Available from 4.13.
	Tag string
	// Name as supplied by user space at load time. Available from 4.15.
	Name string

	btf   btf.ID
	stats *programStats

	maps  []MapID
	insns []byte
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
			runtime:  time.Duration(info.RunTimeNs),
			runCount: info.RunCnt,
		},
	}

	// Start with a clean struct for the second call, otherwise we may get EFAULT.
	var info2 sys.ProgInfo

	if info.NrMapIds > 0 {
		pi.maps = make([]MapID, info.NrMapIds)
		info2.NrMapIds = info.NrMapIds
		info2.MapIds = sys.NewPointer(unsafe.Pointer(&pi.maps[0]))
	}

	if info.XlatedProgLen > 0 {
		pi.insns = make([]byte, info.XlatedProgLen)
		info2.XlatedProgLen = info.XlatedProgLen
		info2.XlatedProgInsns = sys.NewSlicePointer(pi.insns)
	}

	if info.NrMapIds > 0 || info.XlatedProgLen > 0 {
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
	if errors.Is(err, errMissingFields) {
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

// BTFID returns the BTF ID associated with the program.
//
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

// Instructions returns the 'xlated' instruction stream of the program
// after it has been verified and rewritten by the kernel. These instructions
// cannot be loaded back into the kernel as-is, this is mainly used for
// inspecting loaded programs for troubleshooting, dumping, etc.
//
// For example, map accesses are made to reference their kernel map IDs,
// not the FDs they had when the program was inserted.
//
// The first instruction is marked as a symbol using the Program's name.
//
// Available from 4.13. Requires CAP_BPF or equivalent.
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

	// Tag the first instruction with the name of the program, if available.
	insns[0] = insns[0].Sym(pi.Name)

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

var errMissingFields = errors.New("missing fields")

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

		if n, err := fmt.Sscanln(parts[1], field); err != nil || n != 1 {
			return fmt.Errorf("can't parse field %s: %v", name, err)
		}

		scanned++
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if len(fields) > 0 && scanned == 0 {
		return ErrNotSupported
	}

	if scanned != len(fields) {
		return errMissingFields
	}

	return nil
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
