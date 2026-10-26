package features

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/sys"
	"github.com/cilium/ebpf/internal/unix"
)

// HaveProgramType probes the running kernel for the availability of the specified program type.
//
// See the package documentation for the meaning of the error return value.
func HaveProgramType(pt ebpf.ProgramType) (err error) {
	return haveProgramTypeMatrix.Result(pt)
}

func probeProgram(spec *ebpf.ProgramSpec) error {
	if spec.Instructions == nil {
		spec.Instructions = asm.Instructions{
			asm.LoadImm(asm.R0, 0, asm.DWord),
			asm.Return(),
		}
	}
	prog, err := ebpf.NewProgramWithOptions(spec, ebpf.ProgramOptions{
		LogDisabled: true,
	})
	if err == nil {
		prog.Close()
	}

	switch {
	// EINVAL occurs when attempting to create a program with an unknown type.
	// E2BIG occurs when ProgLoadAttr contains non-zero bytes past the end
	// of the struct known by the running kernel, meaning the kernel is too old
	// to support the given prog type.
	case errors.Is(err, unix.EINVAL), errors.Is(err, unix.E2BIG):
		err = ebpf.ErrNotSupported
	}

	return err
}

var haveProgramTypeMatrix = internal.FeatureMatrix[ebpf.ProgramType]{
	ebpf.SocketFilter:  {Version: "3.19"},
	ebpf.Kprobe:        {Version: "4.1"},
	ebpf.SchedCLS:      {Version: "4.1"},
	ebpf.SchedACT:      {Version: "4.1"},
	ebpf.TracePoint:    {Version: "4.7"},
	ebpf.XDP:           {Version: "4.8"},
	ebpf.PerfEvent:     {Version: "4.9"},
	ebpf.CGroupSKB:     {Version: "4.10"},
	ebpf.CGroupSock:    {Version: "4.10"},
	ebpf.LWTIn:         {Version: "4.10"},
	ebpf.LWTOut:        {Version: "4.10"},
	ebpf.LWTXmit:       {Version: "4.10"},
	ebpf.SockOps:       {Version: "4.13"},
	ebpf.SkSKB:         {Version: "4.14"},
	ebpf.CGroupDevice:  {Version: "4.15"},
	ebpf.SkMsg:         {Version: "4.17"},
	ebpf.RawTracepoint: {Version: "4.17"},
	ebpf.CGroupSockAddr: {
		Version: "4.17",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:       ebpf.CGroupSockAddr,
				AttachType: ebpf.AttachCGroupInet4Connect,
			})
		},
	},
	ebpf.LWTSeg6Local:          {Version: "4.18"},
	ebpf.LircMode2:             {Version: "4.18"},
	ebpf.SkReuseport:           {Version: "4.19"},
	ebpf.FlowDissector:         {Version: "4.20"},
	ebpf.CGroupSysctl:          {Version: "5.2"},
	ebpf.RawTracepointWritable: {Version: "5.2"},
	ebpf.CGroupSockopt: {
		Version: "5.3",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:       ebpf.CGroupSockopt,
				AttachType: ebpf.AttachCGroupGetsockopt,
			})
		},
	},
	ebpf.Tracing: {
		Version: "5.5",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:       ebpf.Tracing,
				AttachType: ebpf.AttachTraceFEntry,
				AttachTo:   "bpf_init",
			})
		},
	},
	ebpf.StructOps: {
		Version: "5.6",
		Fn: func() error {
			err := probeProgram(&ebpf.ProgramSpec{
				Type:    ebpf.StructOps,
				License: "GPL",
			})
			if errors.Is(err, sys.ENOTSUPP) {
				// ENOTSUPP means the program type is at least known to the kernel.
				return nil
			}
			return err
		},
	},
	ebpf.Extension: {
		Version: "5.6",
		Fn: func() error {
			// create btf.Func to add to first ins of target and extension so both progs are btf powered
			btfFn := btf.Func{
				Name: "a",
				Type: &btf.FuncProto{
					Return: &btf.Int{},
					Params: []btf.FuncParam{
						{Name: "ctx", Type: &btf.Pointer{Target: &btf.Struct{Name: "xdp_md"}}},
					},
				},
				Linkage: btf.GlobalFunc,
			}
			insns := asm.Instructions{
				btf.WithFuncMetadata(asm.Mov.Imm(asm.R0, 0), &btfFn),
				asm.Return(),
			}

			// create target prog
			prog, err := ebpf.NewProgramWithOptions(
				&ebpf.ProgramSpec{
					Type:         ebpf.XDP,
					Instructions: insns,
				},
				ebpf.ProgramOptions{
					LogDisabled: true,
				},
			)
			if err != nil {
				return err
			}
			defer prog.Close()

			// probe for Extension prog with target
			return probeProgram(&ebpf.ProgramSpec{
				Type:         ebpf.Extension,
				Instructions: insns,
				AttachTarget: prog,
				AttachTo:     btfFn.Name,
			})
		},
	},
	ebpf.LSM: {
		Version: "5.7",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:       ebpf.LSM,
				AttachType: ebpf.AttachLSMMac,
				AttachTo:   "file_mprotect",
				License:    "GPL",
			})
		},
	},
	ebpf.SkLookup: {
		Version: "5.9",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:       ebpf.SkLookup,
				AttachType: ebpf.AttachSkLookup,
			})
		},
	},
	ebpf.Syscall: {
		Version: "5.14",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:  ebpf.Syscall,
				Flags: sys.BPF_F_SLEEPABLE,
			})
		},
	},
	ebpf.Netfilter: {
		Version: "6.4",
		Fn: func() error {
			return probeProgram(&ebpf.ProgramSpec{
				Type:       ebpf.Netfilter,
				AttachType: ebpf.AttachNetfilter,
			})
		},
	},
}

func init() {
	for key, ft := range haveProgramTypeMatrix {
		ft.Name = key.String()
		if ft.Fn == nil {
			key := key // avoid the dreaded loop variable problem
			ft.Fn = func() error { return probeProgram(&ebpf.ProgramSpec{Type: key}) }
		}
	}
}

type helperKey struct {
	typ    ebpf.ProgramType
	helper asm.BuiltinFunc
}

var helperCache = internal.NewFeatureCache(func(key helperKey) *internal.FeatureTest {
	return &internal.FeatureTest{
		Name: fmt.Sprintf("%s for program type %s", key.helper, key.typ),
		Fn: func() error {
			return haveProgramHelper(key.typ, key.helper)
		},
	}
})

// HaveProgramHelper probes the running kernel for the availability of the specified helper
// function to a specified program type.
// Return values have the following semantics:
//
//	err == nil: The feature is available.
//	errors.Is(err, ebpf.ErrNotSupported): The feature is not available.
//	err != nil: Any errors encountered during probe execution, wrapped.
//
// Note that the latter case may include false negatives, and that program creation may
// succeed despite an error being returned.
// Only `nil` and `ebpf.ErrNotSupported` are conclusive.
//
// Probe results are cached and persist throughout any process capability changes.
func HaveProgramHelper(pt ebpf.ProgramType, helper asm.BuiltinFunc) error {
	return helperCache.Result(helperKey{pt, helper})
}

func haveProgramHelper(pt ebpf.ProgramType, helper asm.BuiltinFunc) error {
	if ok := helperProbeNotImplemented(pt); ok {
		return fmt.Errorf("no feature probe for %v/%v", pt, helper)
	}

	if err := HaveProgramType(pt); err != nil {
		return err
	}

	spec := &ebpf.ProgramSpec{
		Type: pt,
		Instructions: asm.Instructions{
			helper.Call(),
			asm.LoadImm(asm.R0, 0, asm.DWord),
			asm.Return(),
		},
		License: "GPL",
	}

	switch pt {
	case ebpf.CGroupSockAddr:
		spec.AttachType = ebpf.AttachCGroupInet4Connect
	case ebpf.CGroupSockopt:
		spec.AttachType = ebpf.AttachCGroupGetsockopt
	case ebpf.SkLookup:
		spec.AttachType = ebpf.AttachSkLookup
	case ebpf.Syscall:
		spec.Flags = sys.BPF_F_SLEEPABLE
	case ebpf.Netfilter:
		spec.AttachType = ebpf.AttachNetfilter
	}

	prog, err := ebpf.NewProgramWithOptions(spec, ebpf.ProgramOptions{
		LogLevel: 1,
	})
	if err == nil {
		prog.Close()
	}

	var verr *ebpf.VerifierError
	if !errors.As(err, &verr) {
		return err
	}

	helperTag := fmt.Sprintf("#%d", helper)

	switch {
	// EACCES occurs when attempting to create a program probe with a helper
	// while the register args when calling this helper aren't set up properly.
	// We interpret this as the helper being available, because the verifier
	// returns EINVAL if the helper is not supported by the running kernel.
	case errors.Is(err, unix.EACCES):
		err = nil

	// EINVAL occurs when attempting to create a program with an unknown helper.
	case errors.Is(err, unix.EINVAL):
		// https://github.com/torvalds/linux/blob/09a0fa92e5b45e99cf435b2fbf5ebcf889cf8780/kernel/bpf/verifier.c#L10663
		if logContainsAll(verr.Log, "invalid func", helperTag) {
			return ebpf.ErrNotSupported
		}

		// https://github.com/torvalds/linux/blob/09a0fa92e5b45e99cf435b2fbf5ebcf889cf8780/kernel/bpf/verifier.c#L10668
		wrongProgramType := logContainsAll(verr.Log, "program of this type cannot use helper", helperTag)
		// https://github.com/torvalds/linux/blob/59b418c7063d30e0a3e1f592d47df096db83185c/kernel/bpf/verifier.c#L10204
		// 4.9 doesn't include # in verifier output.
		wrongProgramType = wrongProgramType || logContainsAll(verr.Log, "unknown func")
		if wrongProgramType {
			return fmt.Errorf("program of this type cannot use helper: %w", ebpf.ErrNotSupported)
		}
	}

	return err
}

func logContainsAll(log []string, needles ...string) bool {
	first := max(len(log)-5, 0) // Check last 5 lines.
	return slices.ContainsFunc(log[first:], func(line string) bool {
		for _, needle := range needles {
			if !strings.Contains(line, needle) {
				return false
			}
		}
		return true
	})
}

func helperProbeNotImplemented(pt ebpf.ProgramType) bool {
	switch pt {
	case ebpf.Extension, ebpf.LSM, ebpf.StructOps, ebpf.Tracing:
		return true
	}
	return false
}
