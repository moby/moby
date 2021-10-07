package link

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/internal"
)

var (
	uprobeEventsPath = filepath.Join(tracefsPath, "uprobe_events")

	// rgxUprobeSymbol is used to strip invalid characters from the uprobe symbol
	// as they are not allowed to be used as the EVENT token in tracefs.
	rgxUprobeSymbol = regexp.MustCompile("[^a-zA-Z0-9]+")

	uprobeRetprobeBit = struct {
		once  sync.Once
		value uint64
		err   error
	}{}

	uprobeRefCtrOffsetPMUPath = "/sys/bus/event_source/devices/uprobe/format/ref_ctr_offset"
	// elixir.bootlin.com/linux/v5.15-rc7/source/kernel/events/core.c#L9799
	uprobeRefCtrOffsetShift = 32
	haveRefCtrOffsetPMU     = internal.FeatureTest("RefCtrOffsetPMU", "4.20", func() error {
		_, err := os.Stat(uprobeRefCtrOffsetPMUPath)
		if err != nil {
			return internal.ErrNotSupported
		}
		return nil
	})

	// ErrNoSymbol indicates that the given symbol was not found
	// in the ELF symbols table.
	ErrNoSymbol = errors.New("not found")
)

// Executable defines an executable program on the filesystem.
type Executable struct {
	// Path of the executable on the filesystem.
	path string
	// Parsed ELF symbols and dynamic symbols offsets.
	offsets map[string]uint64
}

// UprobeOptions defines additional parameters that will be used
// when loading Uprobes.
type UprobeOptions struct {
	// Symbol offset. Must be provided in case of external symbols (shared libs).
	// If set, overrides the offset eventually parsed from the executable.
	Offset uint64
	// Only set the uprobe on the given process ID. Useful when tracing
	// shared library calls or programs that have many running instances.
	PID int
	// Automatically manage SDT reference counts (semaphores).
	//
	// If this field is set, the Kernel will increment/decrement the
	// semaphore located in the process memory at the provided address on
	// probe attach/detach.
	//
	// See also:
	// sourceware.org/systemtap/wiki/UserSpaceProbeImplementation (Semaphore Handling)
	// github.com/torvalds/linux/commit/1cc33161a83d
	// github.com/torvalds/linux/commit/a6ca88b241d5
	RefCtrOffset uint64
}

// To open a new Executable, use:
//
//  OpenExecutable("/bin/bash")
//
// The returned value can then be used to open Uprobe(s).
func OpenExecutable(path string) (*Executable, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file '%s': %w", path, err)
	}
	defer f.Close()

	se, err := internal.NewSafeELFFile(f)
	if err != nil {
		return nil, fmt.Errorf("parse ELF file: %w", err)
	}

	if se.Type != elf.ET_EXEC && se.Type != elf.ET_DYN {
		// ELF is not an executable or a shared object.
		return nil, errors.New("the given file is not an executable or a shared object")
	}

	ex := Executable{
		path:    path,
		offsets: make(map[string]uint64),
	}

	if err := ex.load(se); err != nil {
		return nil, err
	}

	return &ex, nil
}

func (ex *Executable) load(f *internal.SafeELFFile) error {
	syms, err := f.Symbols()
	if err != nil && !errors.Is(err, elf.ErrNoSymbols) {
		return err
	}

	dynsyms, err := f.DynamicSymbols()
	if err != nil && !errors.Is(err, elf.ErrNoSymbols) {
		return err
	}

	syms = append(syms, dynsyms...)

	for _, s := range syms {
		if elf.ST_TYPE(s.Info) != elf.STT_FUNC {
			// Symbol not associated with a function or other executable code.
			continue
		}

		off := s.Value

		// Loop over ELF segments.
		for _, prog := range f.Progs {
			// Skip uninteresting segments.
			if prog.Type != elf.PT_LOAD || (prog.Flags&elf.PF_X) == 0 {
				continue
			}

			if prog.Vaddr <= s.Value && s.Value < (prog.Vaddr+prog.Memsz) {
				// If the symbol value is contained in the segment, calculate
				// the symbol offset.
				//
				// fn symbol offset = fn symbol VA - .text VA + .text offset
				//
				// stackoverflow.com/a/40249502
				off = s.Value - prog.Vaddr + prog.Off
				break
			}
		}

		ex.offsets[s.Name] = off
	}

	return nil
}

func (ex *Executable) offset(symbol string) (uint64, error) {
	if off, ok := ex.offsets[symbol]; ok {
		// Symbols with location 0 from section undef are shared library calls and
		// are relocated before the binary is executed. Dynamic linking is not
		// implemented by the library, so mark this as unsupported for now.
		//
		// Since only offset values are stored and not elf.Symbol, if the value is 0,
		// assume it's an external symbol.
		if off == 0 {
			return 0, fmt.Errorf("cannot resolve %s library call '%s', "+
				"consider providing the offset via options: %w", ex.path, symbol, ErrNotSupported)
		}
		return off, nil
	}
	return 0, fmt.Errorf("symbol %s: %w", symbol, ErrNoSymbol)
}

// Uprobe attaches the given eBPF program to a perf event that fires when the
// given symbol starts executing in the given Executable.
// For example, /bin/bash::main():
//
//  ex, _ = OpenExecutable("/bin/bash")
//  ex.Uprobe("main", prog, nil)
//
// When using symbols which belongs to shared libraries,
// an offset must be provided via options:
//
//  up, err := ex.Uprobe("main", prog, &UprobeOptions{Offset: 0x123})
//
// Losing the reference to the resulting Link (up) will close the Uprobe
// and prevent further execution of prog. The Link must be Closed during
// program shutdown to avoid leaking system resources.
//
// Functions provided by shared libraries can currently not be traced and
// will result in an ErrNotSupported.
func (ex *Executable) Uprobe(symbol string, prog *ebpf.Program, opts *UprobeOptions) (Link, error) {
	u, err := ex.uprobe(symbol, prog, opts, false)
	if err != nil {
		return nil, err
	}

	err = u.attach(prog)
	if err != nil {
		u.Close()
		return nil, err
	}

	return u, nil
}

// Uretprobe attaches the given eBPF program to a perf event that fires right
// before the given symbol exits. For example, /bin/bash::main():
//
//  ex, _ = OpenExecutable("/bin/bash")
//  ex.Uretprobe("main", prog, nil)
//
// When using symbols which belongs to shared libraries,
// an offset must be provided via options:
//
//  up, err := ex.Uretprobe("main", prog, &UprobeOptions{Offset: 0x123})
//
// Losing the reference to the resulting Link (up) will close the Uprobe
// and prevent further execution of prog. The Link must be Closed during
// program shutdown to avoid leaking system resources.
//
// Functions provided by shared libraries can currently not be traced and
// will result in an ErrNotSupported.
func (ex *Executable) Uretprobe(symbol string, prog *ebpf.Program, opts *UprobeOptions) (Link, error) {
	u, err := ex.uprobe(symbol, prog, opts, true)
	if err != nil {
		return nil, err
	}

	err = u.attach(prog)
	if err != nil {
		u.Close()
		return nil, err
	}

	return u, nil
}

// uprobe opens a perf event for the given binary/symbol and attaches prog to it.
// If ret is true, create a uretprobe.
func (ex *Executable) uprobe(symbol string, prog *ebpf.Program, opts *UprobeOptions, ret bool) (*perfEvent, error) {
	if prog == nil {
		return nil, fmt.Errorf("prog cannot be nil: %w", errInvalidInput)
	}
	if prog.Type() != ebpf.Kprobe {
		return nil, fmt.Errorf("eBPF program type %s is not Kprobe: %w", prog.Type(), errInvalidInput)
	}
	if opts == nil {
		opts = &UprobeOptions{}
	}

	offset := opts.Offset
	if offset == 0 {
		off, err := ex.offset(symbol)
		if err != nil {
			return nil, err
		}
		offset = off
	}

	pid := opts.PID
	if pid == 0 {
		pid = perfAllThreads
	}

	if opts.RefCtrOffset != 0 {
		if err := haveRefCtrOffsetPMU(); err != nil {
			return nil, fmt.Errorf("uprobe ref_ctr_offset: %w", err)
		}
	}

	args := probeArgs{
		symbol:       symbol,
		path:         ex.path,
		offset:       offset,
		pid:          pid,
		refCtrOffset: opts.RefCtrOffset,
		ret:          ret,
	}

	// Use uprobe PMU if the kernel has it available.
	tp, err := pmuUprobe(args)
	if err == nil {
		return tp, nil
	}
	if err != nil && !errors.Is(err, ErrNotSupported) {
		return nil, fmt.Errorf("creating perf_uprobe PMU: %w", err)
	}

	// Use tracefs if uprobe PMU is missing.
	args.symbol = uprobeSanitizedSymbol(symbol)
	tp, err = tracefsUprobe(args)
	if err != nil {
		return nil, fmt.Errorf("creating trace event '%s:%s' in tracefs: %w", ex.path, symbol, err)
	}

	return tp, nil
}

// pmuUprobe opens a perf event based on the uprobe PMU.
func pmuUprobe(args probeArgs) (*perfEvent, error) {
	return pmuProbe(uprobeType, args)
}

// tracefsUprobe creates a Uprobe tracefs entry.
func tracefsUprobe(args probeArgs) (*perfEvent, error) {
	return tracefsProbe(uprobeType, args)
}

// uprobeSanitizedSymbol replaces every invalid characted for the tracefs api with an underscore.
func uprobeSanitizedSymbol(symbol string) string {
	return rgxUprobeSymbol.ReplaceAllString(symbol, "_")
}

// uprobeToken creates the PATH:OFFSET(REF_CTR_OFFSET) token for the tracefs api.
func uprobeToken(args probeArgs) string {
	po := fmt.Sprintf("%s:%#x", args.path, args.offset)

	if args.refCtrOffset != 0 {
		// This is not documented in Documentation/trace/uprobetracer.txt.
		// elixir.bootlin.com/linux/v5.15-rc7/source/kernel/trace/trace.c#L5564
		po += fmt.Sprintf("(%#x)", args.refCtrOffset)
	}

	return po
}

func uretprobeBit() (uint64, error) {
	uprobeRetprobeBit.once.Do(func() {
		uprobeRetprobeBit.value, uprobeRetprobeBit.err = determineRetprobeBit(uprobeType)
	})
	return uprobeRetprobeBit.value, uprobeRetprobeBit.err
}
