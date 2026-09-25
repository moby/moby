package features

import (
	"errors"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// HaveBPFLinkUprobeMulti probes the running kernel if uprobe_multi link is supported.
//
// See the package documentation for the meaning of the error return value.
func HaveBPFLinkUprobeMulti() error {
	return haveBPFLinkUprobeMulti()
}

var haveBPFLinkUprobeMulti = internal.NewFeatureTest("bpf_link_uprobe_multi", func() error {
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name: "probe_upm_link",
		Type: ebpf.Kprobe,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
		AttachType: ebpf.AttachTraceUprobeMulti,
		License:    "MIT",
	})
	if errors.Is(err, unix.E2BIG) {
		// Kernel doesn't support AttachType field.
		return ebpf.ErrNotSupported
	}
	if err != nil {
		return err
	}
	defer prog.Close()

	// We try to create uprobe multi link on '/' path which results in
	// error with -EBADF in case uprobe multi link is supported.
	fd, err := sys.LinkCreateUprobeMulti(&sys.LinkCreateUprobeMultiAttr{
		ProgFd:     uint32(prog.FD()),
		AttachType: sys.BPF_TRACE_UPROBE_MULTI,
		Path:       sys.NewStringPointer("/"),
		Offsets:    sys.SlicePointer([]uint64{0}),
		Count:      1,
	})
	switch {
	case errors.Is(err, unix.EBADF):
		return nil
	case errors.Is(err, unix.EINVAL):
		return ebpf.ErrNotSupported
	case err != nil:
		return err
	}

	// should not happen
	fd.Close()
	return errors.New("successfully attached uprobe_multi to /, kernel bug?")
}, "6.6")

// HaveBPFLinkKprobeMulti probes the running kernel if kprobe_multi link is supported.
//
// See the package documentation for the meaning of the error return value.
func HaveBPFLinkKprobeMulti() error {
	return haveBPFLinkKprobeMulti()
}

var haveBPFLinkKprobeMulti = internal.NewFeatureTest("bpf_link_kprobe_multi", func() error {
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name: "probe_kpm_link",
		Type: ebpf.Kprobe,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
		AttachType: ebpf.AttachTraceKprobeMulti,
		License:    "MIT",
	})
	if errors.Is(err, unix.E2BIG) {
		// Kernel doesn't support AttachType field.
		return ebpf.ErrNotSupported
	}
	if err != nil {
		return err
	}
	defer prog.Close()

	fd, err := sys.LinkCreateKprobeMulti(&sys.LinkCreateKprobeMultiAttr{
		ProgFd:     uint32(prog.FD()),
		AttachType: sys.BPF_TRACE_KPROBE_MULTI,
		Count:      1,
		Syms:       sys.NewStringSlicePointer([]string{"vprintk"}),
	})
	switch {
	case errors.Is(err, unix.EINVAL):
		return ebpf.ErrNotSupported
	// If CONFIG_FPROBE isn't set.
	case errors.Is(err, unix.EOPNOTSUPP):
		return ebpf.ErrNotSupported
	case err != nil:
		return err
	}

	fd.Close()

	return nil
}, "5.18")

// HaveBPFLinkKprobeSession probes the running kernel if kprobe_session link is supported.
//
// See the package documentation for the meaning of the error return value.
func HaveBPFLinkKprobeSession() error {
	return haveBPFLinkKprobeSession()
}

var haveBPFLinkKprobeSession = internal.NewFeatureTest("bpf_link_kprobe_session", func() error {
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name: "probe_kps_link",
		Type: ebpf.Kprobe,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
		AttachType: ebpf.AttachTraceKprobeSession,
		License:    "MIT",
	})
	if errors.Is(err, unix.E2BIG) {
		// Kernel doesn't support AttachType field.
		return ebpf.ErrNotSupported
	}
	if err != nil {
		return err
	}
	defer prog.Close()

	fd, err := sys.LinkCreateKprobeMulti(&sys.LinkCreateKprobeMultiAttr{
		ProgFd:     uint32(prog.FD()),
		AttachType: sys.BPF_TRACE_KPROBE_SESSION,
		Count:      1,
		Syms:       sys.NewStringSlicePointer([]string{"vprintk"}),
	})
	switch {
	case errors.Is(err, unix.EINVAL):
		return ebpf.ErrNotSupported
	// If CONFIG_FPROBE isn't set.
	case errors.Is(err, unix.EOPNOTSUPP):
		return ebpf.ErrNotSupported
	case err != nil:
		return err
	}

	fd.Close()

	return nil
}, "6.10")
