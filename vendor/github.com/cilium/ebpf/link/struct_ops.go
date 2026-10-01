package link

import (
	"fmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/internal/sys"
)

type structOpsLink struct {
	RawLink
}

func (*structOpsLink) Update(*ebpf.Program) error {
	return fmt.Errorf("update struct_ops link: %w", ErrNotSupported)
}

type StructOpsOptions struct {
	Map *ebpf.Map
}

// AttachStructOps attaches a struct_ops map (created from a ".struct_ops.link"
// section) to its kernel subsystem via a BPF link.
func AttachStructOps(opts StructOpsOptions) (Link, error) {
	m := opts.Map

	if m == nil {
		return nil, fmt.Errorf("map cannot be nil")
	}

	if t := m.Type(); t != ebpf.StructOpsMap {
		return nil, fmt.Errorf("can't attach non-struct_ops map")
	}

	mapFD := m.FD()
	if mapFD <= 0 {
		return nil, fmt.Errorf("invalid map: %s", sys.ErrClosedFd)
	}

	fd, err := sys.LinkCreate(&sys.LinkCreateAttr{
		// For struct_ops links, the mapFD must be passed as ProgFd.
		ProgFd:     uint32(mapFD),
		AttachType: sys.AttachType(ebpf.AttachStructOps),
		TargetFd:   0,
	})
	if err != nil {
		return nil, fmt.Errorf("attach StructOps: create link: %w", err)
	}

	return &structOpsLink{RawLink{fd: fd}}, nil
}
