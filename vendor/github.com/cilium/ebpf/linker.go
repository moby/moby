package ebpf

import (
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"slices"
	"strings"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/kallsyms"
	"github.com/cilium/ebpf/internal/platform"
)

// handles stores handle objects to avoid gc cleanup
type handles []*btf.Handle

func (hs *handles) add(h *btf.Handle) (int, error) {
	if h == nil {
		return 0, nil
	}

	if len(*hs) == math.MaxInt16 {
		return 0, fmt.Errorf("can't add more than %d module FDs to fdArray", math.MaxInt16)
	}

	*hs = append(*hs, h)

	// return length of slice so that indexes start at 1
	return len(*hs), nil
}

func (hs handles) fdArray() []int32 {
	// first element of fda is reserved as no module can be indexed with 0
	fda := []int32{0}
	for _, h := range hs {
		fda = append(fda, int32(h.FD()))
	}

	return fda
}

func (hs *handles) Close() error {
	var errs []error
	for _, h := range *hs {
		errs = append(errs, h.Close())
	}
	return errors.Join(errs...)
}

// The linker is responsible for resolving bpf-to-bpf calls between programs
// within an ELF. Each BPF program must be a self-contained binary blob,
// so when an instruction in one ELF program section wants to jump to
// a function in another, the linker needs to pull in the bytecode
// (and BTF info) of the target function and concatenate the instruction
// streams.
//
// Later on in the pipeline, all call sites are fixed up with relative jumps
// within this newly-created instruction stream to then finally hand off to
// the kernel with BPF_PROG_LOAD.
//
// Each function is denoted by an ELF symbol and the compiler takes care of
// register setup before each jump instruction.

// hasFunctionReferences returns true if insns contains one or more bpf2bpf
// function references.
func hasFunctionReferences(insns asm.Instructions) bool {
	for _, i := range insns {
		if i.IsFunctionReference() {
			return true
		}
	}
	return false
}

// applyRelocations collects and applies any CO-RE relocations in insns.
//
// insns are modified in place.
func applyRelocations(insns asm.Instructions, bo binary.ByteOrder, b *btf.Builder, c *btf.Cache, kernelOverride *btf.Spec, extraTargets []*btf.Spec) error {
	var relos []*btf.CORERelocation
	var reloInsns []*asm.Instruction
	iter := insns.Iterate()
	for iter.Next() {
		if relo := btf.CORERelocationMetadata(iter.Ins); relo != nil {
			relos = append(relos, relo)
			reloInsns = append(reloInsns, iter.Ins)
		}
	}

	if len(relos) == 0 {
		return nil
	}

	if bo == nil {
		bo = internal.NativeEndian
	}

	var targets []*btf.Spec
	if kernelOverride == nil {
		kernel, err := c.Kernel()
		if err != nil {
			return fmt.Errorf("load kernel spec: %w", err)
		}

		modules, err := c.Modules()
		// Ignore ErrNotExists to cater to kernels which have CONFIG_DEBUG_INFO_BTF_MODULES
		// or CONFIG_DEBUG_INFO_BTF disabled.
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		targets = make([]*btf.Spec, 0, 1+len(modules)+len(extraTargets))
		targets = append(targets, kernel)

		for _, kmod := range modules {
			spec, err := c.Module(kmod)
			if err != nil {
				return fmt.Errorf("load BTF for kmod %s: %w", kmod, err)
			}

			targets = append(targets, spec)
		}
	} else {
		// We expect kernelOverride to contain the merged types
		// of vmlinux and kernel modules, as distributed by btfhub.
		targets = []*btf.Spec{kernelOverride}
	}

	targets = append(targets, extraTargets...)

	fixups, err := btf.CORERelocate(relos, targets, bo, b.Add)
	if err != nil {
		return err
	}

	for i, fixup := range fixups {
		if err := fixup.Apply(reloInsns[i]); err != nil {
			return fmt.Errorf("fixup for %s: %w", relos[i], err)
		}
	}

	return nil
}

// flattenPrograms resolves bpf-to-bpf calls for a set of programs.
//
// Links all programs in names by modifying their ProgramSpec in progs.
func flattenPrograms(progs map[string]*ProgramSpec, names []string) {
	// Pre-calculate all function references.
	refs := make(map[*ProgramSpec][]string)
	for _, prog := range progs {
		refs[prog] = prog.Instructions.FunctionReferences()
	}

	// Create a flattened instruction stream, but don't modify progs yet to
	// avoid linking multiple times.
	flattened := make([]asm.Instructions, 0, len(names))
	for _, name := range names {
		flattened = append(flattened, flattenInstructions(name, progs, refs))
	}

	// Finally, assign the flattened instructions.
	for i, name := range names {
		progs[name].Instructions = flattened[i]
	}
}

// flattenInstructions resolves bpf-to-bpf calls for a single program.
//
// Flattens the instructions of prog by concatenating the instructions of all
// direct and indirect dependencies.
//
// progs contains all referenceable programs, while refs contain the direct
// dependencies of each program.
func flattenInstructions(name string, progs map[string]*ProgramSpec, refs map[*ProgramSpec][]string) asm.Instructions {
	prog := progs[name]
	progRefs := refs[prog]

	if len(progRefs) == 0 {
		// No references, nothing to do.
		return prog.Instructions
	}

	insns := make(asm.Instructions, len(prog.Instructions))
	copy(insns, prog.Instructions)

	// Add all direct references of prog to the list of to be linked programs.
	pending := make([]string, len(progRefs))
	copy(pending, progRefs)

	// All references for which we've appended instructions.
	linked := make(map[string]bool)

	// Iterate all pending references. We can't use a range since pending is
	// modified in the body below.
	for len(pending) > 0 {
		var ref string
		ref, pending = pending[0], pending[1:]

		if linked[ref] {
			// We've already linked this ref, don't append instructions again.
			continue
		}

		progRef := progs[ref]
		if progRef == nil {
			// We don't have instructions that go with this reference. This
			// happens when calling extern functions.
			continue
		}

		insns = append(insns, progRef.Instructions...)
		linked[ref] = true

		// Make sure we link indirect references.
		pending = append(pending, refs[progRef]...)
	}

	return insns
}

// fixupAndValidate is called by the ELF reader right before marshaling the
// instruction stream. It performs last-minute adjustments to the program and
// runs some sanity checks before sending it off to the kernel.
func fixupAndValidate(insns asm.Instructions) error {
	iter := insns.Iterate()
	for iter.Next() {
		ins := iter.Ins

		// Map load was tagged with a Reference, but does not contain a Map pointer.
		needsMap := ins.Reference() != "" || ins.Metadata.Get(kconfigMetaKey{}) != nil
		if ins.IsLoadFromMap() && needsMap && ins.Map() == nil {
			return fmt.Errorf("instruction %d: %w", iter.Index, asm.ErrUnsatisfiedMapReference)
		}

		fixupProbeReadKernel(ins)
	}

	return nil
}

// A constant used to poison calls to non-existent kfuncs.
//
// Similar POISON_CALL_KFUNC_BASE in libbpf, except that we use a value lower
// than 2^28 to fit into a tagged constant.
const kfuncCallPoisonBase = 0xdedc0de

// fixupKfuncs loops over all instructions in search for kfunc calls.
// If at least one is found, the current kernels BTF and module BTFis are searched to set Instruction.Constant
// and Instruction.Offset to the correct values.
func fixupKfuncs(insns asm.Instructions, cache *btf.Cache) (_ handles, err error) {
	closeOnError := func(c io.Closer) {
		if err != nil {
			c.Close()
		}
	}

	iter := insns.Iterate()
	for iter.Next() {
		ins := iter.Ins
		if metadata := ins.Metadata.Get(kfuncMetaKey{}); metadata != nil {
			goto fixups
		}
	}

	return nil, nil

fixups:
	// Only load kernel BTF if we found at least one kfunc call. kernelSpec can be
	// nil if the kernel does not have BTF, in which case we poison all kfunc
	// calls.
	_, err = cache.Kernel()
	// ErrNotSupportedOnOS wraps ErrNotSupported, check for it first.
	if errors.Is(err, internal.ErrNotSupportedOnOS) {
		return nil, fmt.Errorf("kfuncs are not supported on this platform: %w", err)
	}
	if err != nil && !errors.Is(err, ErrNotSupported) {
		return nil, err
	}

	fdArray := make(handles, 0)
	defer closeOnError(&fdArray)

	for {
		ins := iter.Ins

		metadata := ins.Metadata.Get(kfuncMetaKey{})
		if metadata == nil {
			if !iter.Next() {
				// break loop if this was the last instruction in the stream.
				break
			}
			continue
		}

		// check meta, if no meta return err
		kfm, _ := metadata.(*kfuncMeta)
		if kfm == nil {
			return nil, fmt.Errorf("kfuncMetaKey doesn't contain kfuncMeta")
		}

		// findTargetInKernel returns [btf.ErrNotFound] if the target can't be found
		// or if BTF is not enabled.
		target := btf.Type((*btf.Func)(nil))
		spec, module, err := findTargetInKernel(kfm.Func.Name, &target, cache)
		if errors.Is(err, btf.ErrNotFound) {
			if kfm.Binding == elf.STB_WEAK {
				if ins.IsKfuncCall() {
					// If the kfunc call is weak and not found, poison the call. Use a
					// recognizable constant to make it easier to debug.
					fn, err := asm.BuiltinFuncForPlatform(platform.Native, kfuncCallPoisonBase)
					if err != nil {
						return nil, err
					}
					*ins = fn.Call()
				} else if ins.OpCode.IsDWordLoad() {
					// If the kfunc DWordLoad is weak and not found, set its address to 0.
					ins.Constant = 0
					ins.Src = 0
				} else {
					return nil, fmt.Errorf("only kfunc calls and dword loads may have kfunc metadata")
				}

				iter.Next()
				continue
			}

			// Error on non-weak kfunc not found.
			return nil, fmt.Errorf("kfunc %q: %w", kfm.Func.Name, ErrNotSupported)
		}
		if err != nil {
			return nil, fmt.Errorf("finding kfunc in kernel: %w", err)
		}

		idx, err := fdArray.add(module)
		if err != nil {
			return nil, err
		}

		if err := btf.CheckTypeCompatibility(kfm.Func.Type, target.(*btf.Func).Type); err != nil {
			return nil, &incompatibleKfuncError{kfm.Func.Name, err}
		}

		id, err := spec.TypeID(target)
		if err != nil {
			return nil, err
		}

		ins.Constant = int64(id)
		ins.Offset = int16(idx)

		if !iter.Next() {
			break
		}
	}

	return fdArray, nil
}

type incompatibleKfuncError struct {
	name string
	err  error
}

func (ike *incompatibleKfuncError) Error() string {
	return fmt.Sprintf("kfunc %q: %s", ike.name, ike.err)
}

// fixupProbeReadKernel replaces calls to bpf_probe_read_{kernel,user}(_str)
// with bpf_probe_read(_str) on kernels that don't support it yet.
func fixupProbeReadKernel(ins *asm.Instruction) {
	if !ins.IsBuiltinCall() {
		return
	}

	// Kernel supports bpf_probe_read_kernel, nothing to do.
	if haveProbeReadKernel() == nil {
		return
	}

	switch asm.BuiltinFunc(ins.Constant) {
	case asm.FnProbeReadKernel, asm.FnProbeReadUser:
		ins.Constant = int64(asm.FnProbeRead)
	case asm.FnProbeReadKernelStr, asm.FnProbeReadUserStr:
		ins.Constant = int64(asm.FnProbeReadStr)
	}
}

// resolveKconfigReferences creates and populates a .kconfig map if necessary.
//
// Returns a nil Map and no error if no references exist.
func resolveKconfigReferences(insns asm.Instructions) (_ *Map, err error) {
	closeOnError := func(c io.Closer) {
		if err != nil {
			c.Close()
		}
	}

	var spec *MapSpec
	iter := insns.Iterate()
	for iter.Next() {
		meta, _ := iter.Ins.Metadata.Get(kconfigMetaKey{}).(*kconfigMeta)
		if meta != nil {
			spec = meta.Map
			break
		}
	}

	if spec == nil {
		return nil, nil
	}

	cpy := spec.Copy()
	if err := resolveKconfig(cpy); err != nil {
		return nil, err
	}

	kconfig, err := NewMap(cpy)
	if err != nil {
		return nil, err
	}
	defer closeOnError(kconfig)

	// Resolve all instructions which load from .kconfig map with actual map
	// and offset inside it.
	iter = insns.Iterate()
	for iter.Next() {
		meta, _ := iter.Ins.Metadata.Get(kconfigMetaKey{}).(*kconfigMeta)
		if meta == nil {
			continue
		}

		if meta.Map != spec {
			return nil, fmt.Errorf("instruction %d: reference to multiple .kconfig maps is not allowed", iter.Index)
		}

		if err := iter.Ins.AssociateMap(kconfig); err != nil {
			return nil, fmt.Errorf("instruction %d: %w", iter.Index, err)
		}

		// Encode a map read at the offset of the var in the datasec.
		iter.Ins.Constant = int64(uint64(meta.Offset) << 32)
		iter.Ins.Metadata.Set(kconfigMetaKey{}, nil)
	}

	return kconfig, nil
}

func resolveKsymReferences(insns asm.Instructions) error {
	type fixup struct {
		*asm.Instruction
		*ksymMeta
	}

	var symbols map[string]uint64
	var fixups []fixup

	iter := insns.Iterate()
	for iter.Next() {
		ins := iter.Ins
		meta, _ := ins.Metadata.Get(ksymMetaKey{}).(*ksymMeta)
		if meta == nil {
			continue
		}

		if symbols == nil {
			symbols = make(map[string]uint64)
		}

		symbols[meta.Name] = 0
		fixups = append(fixups, fixup{
			iter.Ins, meta,
		})
	}

	if len(symbols) == 0 {
		return nil
	}

	err := kallsyms.AssignAddresses(symbols)
	// Tolerate ErrRestrictedKernel during initial lookup, user may have all weak
	// ksyms and a fallback path.
	if err != nil && !errors.Is(err, ErrRestrictedKernel) {
		return fmt.Errorf("resolve ksyms: %w", err)
	}

	var missing []string
	for _, fixup := range fixups {
		addr := symbols[fixup.Name]
		// A weak ksym variable in eBPF C means its resolution is optional.
		if addr == 0 && fixup.Binding != elf.STB_WEAK {
			if !slices.Contains(missing, fixup.Name) {
				missing = append(missing, fixup.Name)
			}
			continue
		}

		fixup.Constant = int64(addr)
	}

	if len(missing) > 0 {
		if err != nil {
			// Program contains required ksyms, return the error from above.
			return fmt.Errorf("resolve required ksyms: %s: %w", strings.Join(missing, ","), err)
		}

		return fmt.Errorf("kernel is missing symbol: %s", strings.Join(missing, ","))
	}

	return nil
}
