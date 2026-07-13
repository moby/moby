package kallsyms

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/cilium/ebpf/internal"
)

var errAmbiguousKsym = errors.New("multiple kernel symbols with the same name")

var symAddrs cache[string, uint64]
var symModules cache[string, string]

// Module returns the kernel module providing the given symbol in the kernel, if
// any. Returns an empty string and no error if the symbol is not present in the
// kernel. Only function symbols are considered. Returns an error if multiple
// symbols with the same name were found.
//
// Consider [AssignModules] if you need to resolve multiple symbols, as it will
// only perform one iteration over /proc/kallsyms.
func Module(name string) (string, error) {
	if name == "" {
		return "", nil
	}

	if mod, ok := symModules.Load(name); ok {
		return mod, nil
	}

	request := map[string]string{name: ""}
	if err := AssignModules(request); err != nil {
		return "", err
	}

	return request[name], nil
}

// AssignModules looks up the kernel module providing each given symbol, if any,
// and assigns them to their corresponding values in the symbols map. Only
// function symbols are considered. Results of all lookups are cached,
// successful or otherwise.
//
// Any symbols missing in the kernel are ignored. Returns an error if multiple
// symbols with a given name were found.
func AssignModules(symbols map[string]string) error {
	if !internal.OnLinux {
		return fmt.Errorf("read /proc/kallsyms: %w", internal.ErrNotSupportedOnOS)
	}

	if len(symbols) == 0 {
		return nil
	}

	// Attempt to fetch symbols from cache.
	request := make(map[string]string)
	for name := range symbols {
		if mod, ok := symModules.Load(name); ok {
			symbols[name] = mod
			continue
		}

		// Mark the symbol to be read from /proc/kallsyms.
		request[name] = ""
	}
	if len(request) == 0 {
		// All symbols satisfied from cache.
		return nil
	}

	f, err := os.Open("/proc/kallsyms")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := assignModules(f, request); err != nil {
		return fmt.Errorf("assigning symbol modules: %w", err)
	}

	// Update the cache with the new symbols. Cache all requested symbols, even if
	// they're missing or don't belong to a module.
	for name, mod := range request {
		symModules.Store(name, mod)
		symbols[name] = mod
	}

	return nil
}

// assignModules assigns kernel symbol modules read from f to values requested
// by symbols. Always scans the whole input to make sure the user didn't request
// an ambiguous symbol.
func assignModules(f io.Reader, symbols map[string]string) error {
	if len(symbols) == 0 {
		return nil
	}

	found := make(map[string]struct{})
	r := newReader(f)
	for r.Line() {
		// Only look for function symbols in the kernel's text section (tT).
		s, err, skip := parseSymbol(r, []rune{'t', 'T'})
		if err != nil {
			return fmt.Errorf("parsing kallsyms line: %w", err)
		}
		if skip {
			continue
		}

		if _, requested := symbols[s.name]; !requested {
			continue
		}

		if _, ok := found[s.name]; ok {
			// We've already seen this symbol. Return an error to avoid silently
			// attaching to a symbol in the wrong module. libbpf also rejects
			// referring to ambiguous symbols.
			//
			// We can't simply check if we already have a value for the given symbol,
			// since many won't have an associated kernel module.
			return fmt.Errorf("symbol %s: duplicate found at address 0x%x (module %q): %w",
				s.name, s.addr, s.mod, errAmbiguousKsym)
		}

		symbols[s.name] = s.mod
		found[s.name] = struct{}{}
	}
	if err := r.Err(); err != nil {
		return fmt.Errorf("reading kallsyms: %w", err)
	}

	return nil
}

// Address returns the address of the given symbol in the kernel. Returns 0 and
// no error if the symbol is not present. Returns an error if multiple addresses
// were found for a symbol.
//
// Consider [AssignAddresses] if you need to resolve multiple symbols, as it
// will only perform one iteration over /proc/kallsyms.
func Address(symbol string) (uint64, error) {
	if symbol == "" {
		return 0, nil
	}

	if addr, ok := symAddrs.Load(symbol); ok {
		return addr, nil
	}

	request := map[string]uint64{symbol: 0}
	if err := AssignAddresses(request); err != nil {
		return 0, err
	}

	return request[symbol], nil
}

// AssignAddresses looks up the addresses of the requested symbols in the kernel
// and assigns them to their corresponding values in the symbols map. Results
// of all lookups are cached, successful or otherwise.
//
// Any symbols missing in the kernel are ignored. Returns an error if multiple
// addresses were found for a symbol.
func AssignAddresses(symbols map[string]uint64) error {
	if !internal.OnLinux {
		return fmt.Errorf("read /proc/kallsyms: %w", internal.ErrNotSupportedOnOS)
	}

	if len(symbols) == 0 {
		return nil
	}

	// Attempt to fetch symbols from cache.
	request := make(map[string]uint64)
	for name := range symbols {
		if addr, ok := symAddrs.Load(name); ok {
			symbols[name] = addr
			continue
		}

		// Mark the symbol to be read from /proc/kallsyms.
		request[name] = 0
	}
	if len(request) == 0 {
		// All symbols satisfied from cache.
		return nil
	}

	f, err := os.Open("/proc/kallsyms")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := assignAddresses(f, request); err != nil {
		return fmt.Errorf("loading symbol addresses: %w", err)
	}

	// Update the cache with the new symbols. Cache all requested symbols even if
	// they weren't found, to avoid repeated lookups.
	for name, addr := range request {
		symAddrs.Store(name, addr)
		symbols[name] = addr
	}

	return nil
}

// assignAddresses assigns kernel symbol addresses read from f to values
// requested by symbols. Always scans the whole input to make sure the user
// didn't request an ambiguous symbol.
func assignAddresses(f io.Reader, symbols map[string]uint64) error {
	if len(symbols) == 0 {
		return nil
	}
	r := newReader(f)
	for r.Line() {
		s, err, skip := parseSymbol(r, nil)
		if err != nil {
			return fmt.Errorf("parsing kallsyms line: %w", err)
		}
		if skip {
			continue
		}

		existing, requested := symbols[s.name]
		if existing != 0 {
			// Multiple addresses for a symbol have been found. Return a friendly
			// error to avoid silently attaching to the wrong symbol. libbpf also
			// rejects referring to ambiguous symbols.
			return fmt.Errorf("symbol %s(0x%x): duplicate found at address 0x%x: %w", s.name, existing, s.addr, errAmbiguousKsym)
		}
		if requested {
			symbols[s.name] = s.addr
		}
	}
	if err := r.Err(); err != nil {
		return fmt.Errorf("reading kallsyms: %w", err)
	}

	return nil
}

type ksym struct {
	addr uint64
	name string
	mod  string
}

// parseSymbol parses a line from /proc/kallsyms into an address, type, name and
// module. Skip will be true if the symbol doesn't match any of the given symbol
// types. See `man 1 nm` for all available types.
//
// Example line: `ffffffffc1682010 T nf_nat_init  [nf_nat]`
func parseSymbol(r *reader, types []rune) (s ksym, err error, skip bool) {
	for i := 0; r.Word(); i++ {
		switch i {
		// Address of the symbol.
		case 0:
			s.addr, err = strconv.ParseUint(r.Text(), 16, 64)
			if err != nil {
				return s, fmt.Errorf("parsing address: %w", err), false
			}
		// Type of the symbol. Assume the character is ASCII-encoded by converting
		// it directly to a rune, since it's a fixed field controlled by the kernel.
		case 1:
			if len(types) > 0 && !slices.Contains(types, rune(r.Bytes()[0])) {
				return s, nil, true
			}
		// Name of the symbol.
		case 2:
			s.name = r.Text()
		// Kernel module the symbol is provided by.
		case 3:
			s.mod = strings.Trim(r.Text(), "[]")
		// Ignore any future fields.
		default:
			break
		}
	}

	return
}
