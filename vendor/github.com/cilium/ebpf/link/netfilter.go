//go:build !windows

package link

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/internal/sys"
)

const NetfilterIPDefrag NetfilterAttachFlags = 0 // Enable IP packet defragmentation

type NetfilterAttachFlags uint32

type NetfilterInetHook = sys.NetfilterInetHook

const (
	NetfilterInetPreRouting  = sys.NF_INET_PRE_ROUTING
	NetfilterInetLocalIn     = sys.NF_INET_LOCAL_IN
	NetfilterInetForward     = sys.NF_INET_FORWARD
	NetfilterInetLocalOut    = sys.NF_INET_LOCAL_OUT
	NetfilterInetPostRouting = sys.NF_INET_POST_ROUTING
)

type NetfilterProtocolFamily = sys.NetfilterProtocolFamily

const (
	NetfilterProtoUnspec = sys.NFPROTO_UNSPEC
	NetfilterProtoInet   = sys.NFPROTO_INET // Inet applies to both IPv4 and IPv6
	NetfilterProtoIPv4   = sys.NFPROTO_IPV4
	NetfilterProtoARP    = sys.NFPROTO_ARP
	NetfilterProtoNetdev = sys.NFPROTO_NETDEV
	NetfilterProtoBridge = sys.NFPROTO_BRIDGE
	NetfilterProtoIPv6   = sys.NFPROTO_IPV6
)

type NetfilterOptions struct {
	// Program must be a netfilter BPF program.
	Program *ebpf.Program
	// The protocol family.
	ProtocolFamily NetfilterProtocolFamily
	// The netfilter hook to attach to.
	Hook NetfilterInetHook
	// Priority within hook
	Priority int32
	// Extra link flags
	Flags uint32
	// Netfilter flags
	NetfilterFlags NetfilterAttachFlags
}

type netfilterLink struct {
	RawLink
}

// AttachNetfilter links a netfilter BPF program to a netfilter hook.
func AttachNetfilter(opts NetfilterOptions) (Link, error) {
	if opts.Program == nil {
		return nil, fmt.Errorf("netfilter program is nil")
	}

	if t := opts.Program.Type(); t != ebpf.Netfilter {
		return nil, fmt.Errorf("invalid program type %s, expected netfilter", t)
	}

	progFd := opts.Program.FD()
	if progFd < 0 {
		return nil, fmt.Errorf("invalid program: %s", sys.ErrClosedFd)
	}

	attr := sys.LinkCreateNetfilterAttr{
		ProgFd:         uint32(opts.Program.FD()),
		AttachType:     sys.BPF_NETFILTER,
		Flags:          opts.Flags,
		Pf:             opts.ProtocolFamily,
		Hooknum:        opts.Hook,
		Priority:       opts.Priority,
		NetfilterFlags: uint32(opts.NetfilterFlags),
	}

	fd, err := sys.LinkCreateNetfilter(&attr)
	if err != nil {
		return nil, fmt.Errorf("attach netfilter link: %w", err)
	}

	return &netfilterLink{RawLink{fd, ""}}, nil
}

func (*netfilterLink) Update(_ *ebpf.Program) error {
	return fmt.Errorf("netfilter update: %w", ErrNotSupported)
}

func (nf *netfilterLink) Info() (*Info, error) {
	var info sys.NetfilterLinkInfo
	if err := sys.ObjInfo(nf.fd, &info); err != nil {
		return nil, fmt.Errorf("netfilter link info: %s", err)
	}
	extra := &NetfilterInfo{
		ProtocolFamily: info.Pf,
		Hook:           info.Hooknum,
		Priority:       info.Priority,
		Flags:          info.Flags,
	}

	return &Info{
		info.Type,
		info.Id,
		ebpf.ProgramID(info.ProgId),
		extra,
	}, nil
}

var _ Link = (*netfilterLink)(nil)
