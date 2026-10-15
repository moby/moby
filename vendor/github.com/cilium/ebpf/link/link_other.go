//go:build !windows

package link

import (
	"fmt"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/platform"
	"github.com/cilium/ebpf/internal/sys"
)

// Valid link types.
const (
	UnspecifiedType   = sys.BPF_LINK_TYPE_UNSPEC
	RawTracepointType = sys.BPF_LINK_TYPE_RAW_TRACEPOINT
	TracingType       = sys.BPF_LINK_TYPE_TRACING
	CgroupType        = sys.BPF_LINK_TYPE_CGROUP
	IterType          = sys.BPF_LINK_TYPE_ITER
	NetNsType         = sys.BPF_LINK_TYPE_NETNS
	XDPType           = sys.BPF_LINK_TYPE_XDP
	PerfEventType     = sys.BPF_LINK_TYPE_PERF_EVENT
	KprobeMultiType   = sys.BPF_LINK_TYPE_KPROBE_MULTI
	TCXType           = sys.BPF_LINK_TYPE_TCX
	UprobeMultiType   = sys.BPF_LINK_TYPE_UPROBE_MULTI
	NetfilterType     = sys.BPF_LINK_TYPE_NETFILTER
	NetkitType        = sys.BPF_LINK_TYPE_NETKIT
	StructOpsType     = sys.BPF_LINK_TYPE_STRUCT_OPS
)

// AttachRawLink creates a raw link.
func AttachRawLink(opts RawLinkOptions) (*RawLink, error) {
	if err := haveBPFLink(); err != nil {
		return nil, err
	}

	if opts.Target < 0 {
		return nil, fmt.Errorf("invalid target: %s", sys.ErrClosedFd)
	}

	progFd := opts.Program.FD()
	if progFd < 0 {
		return nil, fmt.Errorf("invalid program: %s", sys.ErrClosedFd)
	}

	p, attachType := platform.DecodeConstant(opts.Attach)
	if p != platform.Linux {
		return nil, fmt.Errorf("attach type %s: %w", opts.Attach, internal.ErrNotSupportedOnOS)
	}

	attr := sys.LinkCreateAttr{
		TargetFd:    uint32(opts.Target),
		ProgFd:      uint32(progFd),
		AttachType:  sys.AttachType(attachType),
		TargetBtfId: opts.BTF,
		Flags:       opts.Flags,
	}
	fd, err := sys.LinkCreate(&attr)
	if err != nil {
		return nil, fmt.Errorf("create link: %w", err)
	}

	return &RawLink{fd, ""}, nil
}

// wrap a RawLink in a more specific type if possible.
//
// The function takes ownership of raw and closes it on error.
func wrapRawLink(raw *RawLink) (_ Link, err error) {
	defer func() {
		if err != nil {
			raw.Close()
		}
	}()

	info, err := raw.Info()
	if err != nil {
		return nil, err
	}

	switch info.Type {
	case RawTracepointType:
		return &rawTracepoint{*raw}, nil
	case TracingType:
		return &tracing{*raw}, nil
	case CgroupType:
		return &linkCgroup{*raw}, nil
	case IterType:
		return &Iter{*raw}, nil
	case NetNsType:
		return &NetNsLink{*raw}, nil
	case KprobeMultiType:
		return &kprobeMultiLink{*raw}, nil
	case UprobeMultiType:
		return &uprobeMultiLink{*raw}, nil
	case PerfEventType:
		return &perfEventLink{*raw, nil}, nil
	case TCXType:
		return &tcxLink{*raw}, nil
	case NetfilterType:
		return &netfilterLink{*raw}, nil
	case NetkitType:
		return &netkitLink{*raw}, nil
	case XDPType:
		return &xdpLink{*raw}, nil
	case StructOpsType:
		return &structOpsLink{*raw}, nil
	default:
		return raw, nil
	}
}

type TracingInfo struct {
	AttachType     sys.AttachType
	TargetObjectId uint32
	TargetBtfId    sys.TypeID
}

type CgroupInfo struct {
	CgroupId   uint64
	AttachType sys.AttachType
	_          [4]byte
}

type NetNsInfo struct {
	NetnsInode uint32
	AttachType sys.AttachType
}

type TCXInfo struct {
	Ifindex    uint32
	AttachType sys.AttachType
}

type XDPInfo struct {
	Ifindex uint32
}

type NetfilterInfo struct {
	ProtocolFamily NetfilterProtocolFamily
	Hook           NetfilterInetHook
	Priority       int32
	Flags          uint32
}

type NetkitInfo struct {
	Ifindex    uint32
	AttachType sys.AttachType
}

type RawTracepointInfo struct {
	Name string
}

type KprobeMultiInfo struct {
	// Count is the number of addresses hooked by the kprobe.
	Count   uint32
	Flags   uint32
	Missed  uint64
	addrs   []uint64
	cookies []uint64
}

type KprobeMultiAddress struct {
	Address uint64
	Cookie  uint64
}

// Addresses are the addresses hooked by the kprobe.
func (kpm *KprobeMultiInfo) Addresses() ([]KprobeMultiAddress, bool) {
	if kpm.addrs == nil || len(kpm.addrs) != len(kpm.cookies) {
		return nil, false
	}
	addrs := make([]KprobeMultiAddress, len(kpm.addrs))
	for i := range kpm.addrs {
		addrs[i] = KprobeMultiAddress{
			Address: kpm.addrs[i],
			Cookie:  kpm.cookies[i],
		}
	}
	return addrs, true
}

type UprobeMultiInfo struct {
	Count         uint32
	Flags         uint32
	Missed        uint64
	offsets       []uint64
	cookies       []uint64
	refCtrOffsets []uint64
	// File is the path that the file the uprobe was attached to
	// had at creation time.
	//
	// However, due to various circumstances (differing mount namespaces,
	// file replacement, ...), this path may not point to the same binary
	// the uprobe was originally attached to.
	File string
	pid  uint32
}

type UprobeMultiOffset struct {
	Offset         uint64
	Cookie         uint64
	ReferenceCount uint64
}

// Offsets returns the offsets that the uprobe was attached to along with the related cookies and ref counters.
func (umi *UprobeMultiInfo) Offsets() ([]UprobeMultiOffset, bool) {
	if umi.offsets == nil || len(umi.cookies) != len(umi.offsets) || len(umi.refCtrOffsets) != len(umi.offsets) {
		return nil, false
	}
	var adresses = make([]UprobeMultiOffset, len(umi.offsets))
	for i := range umi.offsets {
		adresses[i] = UprobeMultiOffset{
			Offset:         umi.offsets[i],
			Cookie:         umi.cookies[i],
			ReferenceCount: umi.refCtrOffsets[i],
		}
	}
	return adresses, true
}

// Pid returns the process ID that this uprobe is attached to.
//
// If it does not exist, the uprobe will trigger for all processes.
func (umi *UprobeMultiInfo) Pid() (uint32, bool) {
	return umi.pid, umi.pid > 0
}

const (
	PerfEventUnspecified = sys.BPF_PERF_EVENT_UNSPEC
	PerfEventUprobe      = sys.BPF_PERF_EVENT_UPROBE
	PerfEventUretprobe   = sys.BPF_PERF_EVENT_URETPROBE
	PerfEventKprobe      = sys.BPF_PERF_EVENT_KPROBE
	PerfEventKretprobe   = sys.BPF_PERF_EVENT_KRETPROBE
	PerfEventTracepoint  = sys.BPF_PERF_EVENT_TRACEPOINT
	PerfEventEvent       = sys.BPF_PERF_EVENT_EVENT
)

type PerfEventInfo struct {
	Type  sys.PerfEventType
	extra any
}

func (r *PerfEventInfo) Kprobe() *KprobeInfo {
	e, _ := r.extra.(*KprobeInfo)
	return e
}

func (r *PerfEventInfo) Uprobe() *UprobeInfo {
	e, _ := r.extra.(*UprobeInfo)
	return e
}

func (r *PerfEventInfo) Tracepoint() *TracepointInfo {
	e, _ := r.extra.(*TracepointInfo)
	return e
}

func (r *PerfEventInfo) Event() *EventInfo {
	e, _ := r.extra.(*EventInfo)
	return e
}

type KprobeInfo struct {
	Address  uint64
	Missed   uint64
	Function string
	Offset   uint32
}

type UprobeInfo struct {
	// File is the path that the file the uprobe was attached to
	// had at creation time.
	//
	// However, due to various circumstances (differing mount namespaces,
	// file replacement, ...), this path may not point to the same binary
	// the uprobe was originally attached to.
	File                 string
	Offset               uint32
	Cookie               uint64
	OffsetReferenceCount uint64
}

type TracepointInfo struct {
	Tracepoint string
	Cookie     uint64
}

type EventInfo struct {
	Config uint64
	Type   uint32
	Cookie uint64
}

// Tracing returns tracing type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) Tracing() *TracingInfo {
	e, _ := r.extra.(*TracingInfo)
	return e
}

// Cgroup returns cgroup type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) Cgroup() *CgroupInfo {
	e, _ := r.extra.(*CgroupInfo)
	return e
}

// NetNs returns netns type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) NetNs() *NetNsInfo {
	e, _ := r.extra.(*NetNsInfo)
	return e
}

// XDP returns XDP type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) XDP() *XDPInfo {
	e, _ := r.extra.(*XDPInfo)
	return e
}

// TCX returns TCX type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) TCX() *TCXInfo {
	e, _ := r.extra.(*TCXInfo)
	return e
}

// Netfilter returns netfilter type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) Netfilter() *NetfilterInfo {
	e, _ := r.extra.(*NetfilterInfo)
	return e
}

// Netkit returns netkit type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) Netkit() *NetkitInfo {
	e, _ := r.extra.(*NetkitInfo)
	return e
}

// KprobeMulti returns kprobe-multi type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) KprobeMulti() *KprobeMultiInfo {
	e, _ := r.extra.(*KprobeMultiInfo)
	return e
}

// UprobeMulti returns uprobe-multi type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) UprobeMulti() *UprobeMultiInfo {
	e, _ := r.extra.(*UprobeMultiInfo)
	return e
}

// PerfEvent returns perf-event type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) PerfEvent() *PerfEventInfo {
	e, _ := r.extra.(*PerfEventInfo)
	return e
}

// RawTracepoint returns raw-tracepoint type-specific link info.
//
// Returns nil if the type-specific link info isn't available.
func (r Info) RawTracepoint() *RawTracepointInfo {
	e, _ := r.extra.(*RawTracepointInfo)
	return e
}
