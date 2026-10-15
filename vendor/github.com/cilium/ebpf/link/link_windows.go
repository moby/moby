package link

import (
	"fmt"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/efw"
	"github.com/cilium/ebpf/internal/platform"
	"github.com/cilium/ebpf/internal/sys"
)

// AttachRawLink creates a raw link.
func AttachRawLink(opts RawLinkOptions) (*RawLink, error) {
	if opts.Target != 0 || opts.BTF != 0 || opts.Flags != 0 {
		return nil, fmt.Errorf("specified option(s) %w", internal.ErrNotSupportedOnOS)
	}

	plat, attachType := platform.DecodeConstant(opts.Attach)
	if plat != platform.Windows {
		return nil, fmt.Errorf("attach type %s: %w", opts.Attach, internal.ErrNotSupportedOnOS)
	}

	attachTypeGUID, err := efw.EbpfGetEbpfAttachType(attachType)
	if err != nil {
		return nil, fmt.Errorf("get attach type: %w", err)
	}

	progFd := opts.Program.FD()
	if progFd < 0 {
		return nil, fmt.Errorf("invalid program: %s", sys.ErrClosedFd)
	}

	raw, err := efw.EbpfProgramAttachFds(progFd, attachTypeGUID, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("attach link: %w", err)
	}

	fd, err := sys.NewFD(int(raw))
	if err != nil {
		return nil, err
	}

	return &RawLink{fd: fd}, nil
}

func wrapRawLink(raw *RawLink) (Link, error) {
	return raw, nil
}
