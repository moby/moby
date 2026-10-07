//go:build !windows

package link

import (
	"errors"
	"fmt"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/features"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// UprobeMultiOptions defines additional parameters that will be used
// when opening a UprobeMulti Link.
type UprobeMultiOptions struct {
	// Symbol addresses. If set, overrides the addresses eventually parsed from
	// the executable. Mutually exclusive with UprobeMulti's symbols argument.
	Addresses []uint64

	// Offsets into functions provided by UprobeMulti's symbols argument.
	// For example: to set uprobes to main+5 and _start+10, call UprobeMulti
	// with:
	//     symbols: "main", "_start"
	//     opt.Offsets: 5, 10
	Offsets []uint64

	// Optional list of associated ref counter offsets.
	RefCtrOffsets []uint64

	// Optional list of associated BPF cookies.
	Cookies []uint64

	// Only set the uprobe_multi link on the given process ID, zero PID means
	// system-wide.
	PID uint32
}

func (ex *Executable) UprobeMulti(symbols []string, prog *ebpf.Program, opts *UprobeMultiOptions) (Link, error) {
	return ex.uprobeMulti(symbols, prog, opts, 0)
}

func (ex *Executable) UretprobeMulti(symbols []string, prog *ebpf.Program, opts *UprobeMultiOptions) (Link, error) {

	// The return probe is not limited for symbols entry, so there's no special
	// setup for return uprobes (other than the extra flag). The symbols, opts.Offsets
	// and opts.Addresses arrays follow the same logic as for entry uprobes.
	return ex.uprobeMulti(symbols, prog, opts, sys.BPF_F_UPROBE_MULTI_RETURN)
}

func (ex *Executable) uprobeMulti(symbols []string, prog *ebpf.Program, opts *UprobeMultiOptions, flags uint32) (Link, error) {
	if prog == nil {
		return nil, errors.New("cannot attach a nil program")
	}

	if opts == nil {
		opts = &UprobeMultiOptions{}
	}

	addresses, err := ex.addresses(symbols, opts.Addresses, opts.Offsets)
	if err != nil {
		return nil, err
	}

	addrs := len(addresses)
	cookies := len(opts.Cookies)
	refCtrOffsets := len(opts.RefCtrOffsets)

	if addrs == 0 {
		return nil, fmt.Errorf("field Addresses is required: %w", errInvalidInput)
	}
	if refCtrOffsets > 0 && refCtrOffsets != addrs {
		return nil, fmt.Errorf("field RefCtrOffsets must be exactly Addresses in length: %w", errInvalidInput)
	}
	if cookies > 0 && cookies != addrs {
		return nil, fmt.Errorf("field Cookies must be exactly Addresses in length: %w", errInvalidInput)
	}

	attr := &sys.LinkCreateUprobeMultiAttr{
		Path:             sys.NewStringPointer(ex.path),
		ProgFd:           uint32(prog.FD()),
		AttachType:       sys.BPF_TRACE_UPROBE_MULTI,
		UprobeMultiFlags: flags,
		Count:            uint32(addrs),
		Offsets:          sys.SlicePointer(addresses),
		Pid:              opts.PID,
	}

	if refCtrOffsets != 0 {
		attr.RefCtrOffsets = sys.SlicePointer(opts.RefCtrOffsets)
	}
	if cookies != 0 {
		attr.Cookies = sys.SlicePointer(opts.Cookies)
	}

	fd, err := sys.LinkCreateUprobeMulti(attr)
	if errors.Is(err, unix.ESRCH) {
		return nil, fmt.Errorf("%w (specified pid not found?)", os.ErrNotExist)
	}
	// Since Linux commit 46ba0e49b642 ("bpf: fix multi-uprobe PID filtering
	// logic"), if the provided pid overflows MaxInt32 (turning it negative), the
	// kernel will return EINVAL instead of ESRCH.
	if errors.Is(err, unix.EINVAL) {
		return nil, fmt.Errorf("%w (invalid pid, missing symbol or prog's AttachType not AttachTraceUprobeMulti?)", err)
	}

	if err != nil {
		if haveFeatErr := features.HaveBPFLinkUprobeMulti(); haveFeatErr != nil {
			return nil, haveFeatErr
		}
		return nil, err
	}

	return &uprobeMultiLink{RawLink{fd, ""}}, nil
}

func (ex *Executable) addresses(symbols []string, addresses, offsets []uint64) ([]uint64, error) {
	n := len(symbols)
	if n == 0 {
		n = len(addresses)
	}

	if n == 0 {
		return nil, fmt.Errorf("%w: neither symbols nor addresses given", errInvalidInput)
	}

	if symbols != nil && len(symbols) != n {
		return nil, fmt.Errorf("%w: have %d symbols but want %d", errInvalidInput, len(symbols), n)
	}

	if addresses != nil && len(addresses) != n {
		return nil, fmt.Errorf("%w: have %d addresses but want %d", errInvalidInput, len(addresses), n)
	}

	if offsets != nil && len(offsets) != n {
		return nil, fmt.Errorf("%w: have %d offsets but want %d", errInvalidInput, len(offsets), n)
	}

	results := make([]uint64, 0, n)
	for i := 0; i < n; i++ {
		var sym string
		if symbols != nil {
			sym = symbols[i]
		}

		var addr, off uint64
		if addresses != nil {
			addr = addresses[i]
		}

		if offsets != nil {
			off = offsets[i]
		}

		result, err := ex.address(sym, addr, off)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

type uprobeMultiLink struct {
	RawLink
}

var _ Link = (*uprobeMultiLink)(nil)

func (kml *uprobeMultiLink) Update(_ *ebpf.Program) error {
	return fmt.Errorf("update uprobe_multi: %w", ErrNotSupported)
}

func (kml *uprobeMultiLink) Info() (*Info, error) {
	var info sys.UprobeMultiLinkInfo
	if err := sys.ObjInfo(kml.fd, &info); err != nil {
		return nil, fmt.Errorf("uprobe multi link info: %s", err)
	}
	var (
		path          = make([]byte, info.PathSize)
		refCtrOffsets = make([]uint64, info.Count)
		addrs         = make([]uint64, info.Count)
		cookies       = make([]uint64, info.Count)
	)
	info = sys.UprobeMultiLinkInfo{
		Path:          sys.SlicePointer(path),
		PathSize:      uint32(len(path)),
		Offsets:       sys.SlicePointer(addrs),
		RefCtrOffsets: sys.SlicePointer(refCtrOffsets),
		Cookies:       sys.SlicePointer(cookies),
		Count:         uint32(len(addrs)),
	}
	if err := sys.ObjInfo(kml.fd, &info); err != nil {
		return nil, fmt.Errorf("uprobe multi link info: %s", err)
	}
	if info.Path.IsNil() {
		path = nil
	}
	if info.Cookies.IsNil() {
		cookies = nil
	}
	if info.Offsets.IsNil() {
		addrs = nil
	}
	if info.RefCtrOffsets.IsNil() {
		refCtrOffsets = nil
	}
	extra := &UprobeMultiInfo{
		Count:         info.Count,
		Flags:         info.Flags,
		pid:           info.Pid,
		offsets:       addrs,
		cookies:       cookies,
		refCtrOffsets: refCtrOffsets,
		File:          unix.ByteSliceToString(path),
	}

	return &Info{
		info.Type,
		info.Id,
		ebpf.ProgramID(info.ProgId),
		extra,
	}, nil
}
