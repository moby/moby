package btf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/linux"
	"github.com/cilium/ebpf/internal/platform"
	"github.com/cilium/ebpf/internal/unix"
)

// LoadKernelSpec returns the current kernel's BTF information.
//
// Defaults to /sys/kernel/btf/vmlinux and falls back to scanning the file
// system for vmlinux ELFs. Returns an error wrapping [ErrNotSupported] if BTF
// is not enabled.
//
// Consider using [Cache] instead.
func LoadKernelSpec() (*Spec, error) {
	if platform.IsWindows {
		return nil, internal.ErrNotSupportedOnOS
	}

	fh, err := os.Open("/sys/kernel/btf/vmlinux")
	if err == nil {
		defer fh.Close()

		info, err := fh.Stat()
		if err != nil {
			return nil, fmt.Errorf("stat vmlinux: %w", err)
		}

		// NB: It's not safe to mmap arbitrary files because mmap(2) doesn't
		// guarantee that changes made after mmap are not visible in the mapping.
		//
		// This is not a problem for vmlinux, since it is always a read-only file.
		raw, err := unix.Mmap(int(fh.Fd()), 0, int(info.Size()), unix.PROT_READ, unix.MAP_PRIVATE)
		if err != nil {
			return LoadSplitSpecFromReader(fh, nil)
		}

		spec, err := loadRawSpec(raw, nil)
		if err != nil {
			_ = unix.Munmap(raw)
			return nil, fmt.Errorf("load vmlinux: %w", err)
		}

		runtime.AddCleanup(spec.decoder.sharedBuf, func(b []byte) {
			_ = unix.Munmap(b)
		}, raw)

		return spec, nil
	}

	file, err := findVMLinux()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	spec, err := LoadSpecFromReader(file)
	return spec, err
}

// LoadKernelModuleSpec returns the BTF information for the named kernel module.
//
// Using [Cache.Module] is faster when loading BTF for more than one module.
//
// Defaults to /sys/kernel/btf/<module>.
// Returns an error wrapping ErrNotSupported if BTF is not enabled.
// Returns an error wrapping fs.ErrNotExist if BTF for the specific module doesn't exist.
func LoadKernelModuleSpec(module string) (*Spec, error) {
	base, err := LoadKernelSpec()
	if err != nil {
		return nil, err
	}
	return loadKernelModuleSpec(module, base)
}

func loadKernelModuleSpec(module string, base *Spec) (*Spec, error) {
	if platform.IsWindows {
		return nil, internal.ErrNotSupportedOnOS
	}

	dir, file := filepath.Split(module)
	if dir != "" || filepath.Ext(file) != "" {
		return nil, fmt.Errorf("invalid module name %q", module)
	}

	fh, err := os.Open(filepath.Join("/sys/kernel/btf", module))
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	return LoadSplitSpecFromReader(fh, base)
}

// findVMLinux scans multiple well-known paths for vmlinux kernel images.
func findVMLinux() (*os.File, error) {
	if platform.IsWindows {
		return nil, fmt.Errorf("find vmlinux: %w", internal.ErrNotSupportedOnOS)
	}

	release, err := linux.KernelRelease()
	if err != nil {
		return nil, err
	}

	// use same list of locations as libbpf
	// https://github.com/libbpf/libbpf/blob/9a3a42608dbe3731256a5682a125ac1e23bced8f/src/btf.c#L3114-L3122
	locations := []string{
		"/boot/vmlinux-%s",
		"/lib/modules/%s/vmlinux-%[1]s",
		"/lib/modules/%s/build/vmlinux",
		"/usr/lib/modules/%s/kernel/vmlinux",
		"/usr/lib/debug/boot/vmlinux-%s",
		"/usr/lib/debug/boot/vmlinux-%s.debug",
		"/usr/lib/debug/lib/modules/%s/vmlinux",
	}

	for _, loc := range locations {
		file, err := os.Open(fmt.Sprintf(loc, release))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return file, err
	}

	return nil, fmt.Errorf("no BTF found for kernel version %s: %w", release, internal.ErrNotSupported)
}

// Cache allows to amortise the cost of decoding BTF across multiple call-sites.
//
// It is safe for concurrent use.
type Cache struct {
	mu            sync.RWMutex
	kernelTypes   *Spec
	moduleTypes   map[string]*Spec
	loadedModules []string
}

// NewCache returns an empty Cache.
//
// Pass it to [CollectionOptions] to share kernel BTF across multiple
// Collection loads.
func NewCache() *Cache {
	return &Cache{}
}

// Kernel is equivalent to [LoadKernelSpec], except that repeated calls do
// not copy the Spec.
//
// Returns an error wrapping [ErrNotSupported] if BTF is not enabled.
func (c *Cache) Kernel() (*Spec, error) {
	c.mu.RLock()
	if c.kernelTypes != nil {
		spec := c.kernelTypes
		c.mu.RUnlock()
		return spec, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kernel()
}

func (c *Cache) kernel() (*Spec, error) {
	if c.kernelTypes != nil {
		return c.kernelTypes, nil
	}

	var err error
	c.kernelTypes, err = LoadKernelSpec()
	return c.kernelTypes, err
}

// Module is equivalent to [LoadKernelModuleSpec], except that repeated calls do
// not copy the spec.
//
// All modules also share the return value of [Kernel] as their base.
func (c *Cache) Module(name string) (*Spec, error) {
	c.mu.RLock()
	if spec := c.moduleTypes[name]; spec != nil {
		c.mu.RUnlock()
		return spec, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if spec := c.moduleTypes[name]; spec != nil {
		return spec, nil
	}

	if c.moduleTypes == nil {
		c.moduleTypes = make(map[string]*Spec)
	}

	base, err := c.kernel()
	if err != nil {
		return nil, err
	}

	spec, err := loadKernelModuleSpec(name, base)
	if err != nil {
		return nil, err
	}

	c.moduleTypes[name] = spec
	return spec, nil
}

// Modules returns a sorted list of all loaded modules.
func (c *Cache) Modules() ([]string, error) {
	c.mu.RLock()
	if c.loadedModules != nil {
		modules := slices.Clone(c.loadedModules)
		c.mu.RUnlock()
		return modules, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loadedModules != nil {
		return slices.Clone(c.loadedModules), nil
	}

	btfDir, err := os.Open("/sys/kernel/btf")
	if err != nil {
		return nil, err
	}
	defer btfDir.Close()

	entries, err := btfDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	entries = slices.DeleteFunc(entries, func(s string) bool {
		return s == "vmlinux"
	})

	sort.Strings(entries)
	c.loadedModules = entries
	return slices.Clone(entries), nil
}
