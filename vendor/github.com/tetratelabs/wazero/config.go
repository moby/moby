package wazero

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net"
	"time"

	"github.com/tetratelabs/wazero/api"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/filecache"
	"github.com/tetratelabs/wazero/internal/internalapi"
	"github.com/tetratelabs/wazero/internal/platform"
	internalsock "github.com/tetratelabs/wazero/internal/sock"
	internalsys "github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/sys"
)

// RuntimeConfig controls runtime behavior, with the default implementation as
// NewRuntimeConfig
//
// The example below explicitly limits to Wasm Core 1.0 features as opposed to
// relying on defaults:
//
//	rConfig = wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV1)
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - RuntimeConfig is immutable. Each WithXXX function returns a new instance
//     including the corresponding change.
type RuntimeConfig interface {
	// WithCoreFeatures sets the WebAssembly Core specification features this
	// runtime supports. Defaults to api.CoreFeaturesV2.
	//
	// Example of disabling a specific feature:
	//	features := api.CoreFeaturesV2.SetEnabled(api.CoreFeatureMutableGlobal, false)
	//	rConfig = wazero.NewRuntimeConfig().WithCoreFeatures(features)
	//
	// # Why default to version 2.0?
	//
	// Many compilers that target WebAssembly require features after
	// api.CoreFeaturesV1 by default. For example, TinyGo v0.24+ requires
	// api.CoreFeatureBulkMemoryOperations. To avoid runtime errors, wazero
	// defaults to api.CoreFeaturesV2, even though it is not yet a Web
	// Standard (REC).
	WithCoreFeatures(api.CoreFeatures) RuntimeConfig

	// WithMemoryLimitPages overrides the maximum pages allowed per memory. The
	// default is 65536, allowing 4GB total memory per instance if the maximum is
	// not encoded in a Wasm binary. Setting a value larger than default will panic.
	//
	// This example reduces the largest possible memory size from 4GB to 128KB:
	//	rConfig = wazero.NewRuntimeConfig().WithMemoryLimitPages(2)
	//
	// Note: Wasm has 32-bit memory and each page is 65536 (2^16) bytes. This
	// implies a max of 65536 (2^16) addressable pages.
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#grow-mem
	WithMemoryLimitPages(memoryLimitPages uint32) RuntimeConfig

	// WithMemoryCapacityFromMax eagerly allocates max memory, unless max is
	// not defined. The default is false, which means minimum memory is
	// allocated and any call to grow memory results in re-allocations.
	//
	// This example ensures any memory.grow instruction will never re-allocate:
	//	rConfig = wazero.NewRuntimeConfig().WithMemoryCapacityFromMax(true)
	//
	// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#grow-mem
	//
	// Note: if the memory maximum is not encoded in a Wasm binary, this
	// results in allocating 4GB. See the doc on WithMemoryLimitPages for detail.
	WithMemoryCapacityFromMax(memoryCapacityFromMax bool) RuntimeConfig

	// WithDebugInfoEnabled toggles DWARF based stack traces in the face of
	// runtime errors. Defaults to true.
	//
	// Those who wish to disable this, can like so:
	//
	//	r := wazero.NewRuntimeWithConfig(wazero.NewRuntimeConfig().WithDebugInfoEnabled(false)
	//
	// When disabled, a stack trace message looks like:
	//
	//	wasm stack trace:
	//		.runtime._panic(i32)
	//		.myFunc()
	//		.main.main()
	//		.runtime.run()
	//		._start()
	//
	// When enabled, the stack trace includes source code information:
	//
	//	wasm stack trace:
	//		.runtime._panic(i32)
	//		  0x16e2: /opt/homebrew/Cellar/tinygo/0.26.0/src/runtime/runtime_tinygowasm.go:73:6
	//		.myFunc()
	//		  0x190b: /Users/XXXXX/wazero/internal/testing/dwarftestdata/testdata/main.go:19:7
	//		.main.main()
	//		  0x18ed: /Users/XXXXX/wazero/internal/testing/dwarftestdata/testdata/main.go:4:3
	//		.runtime.run()
	//		  0x18cc: /opt/homebrew/Cellar/tinygo/0.26.0/src/runtime/scheduler_none.go:26:10
	//		._start()
	//		  0x18b6: /opt/homebrew/Cellar/tinygo/0.26.0/src/runtime/runtime_wasm_wasi.go:22:5
	//
	// Note: This only takes into effect when the original Wasm binary has the
	// DWARF "custom sections" that are often stripped, depending on
	// optimization flags passed to the compiler.
	WithDebugInfoEnabled(bool) RuntimeConfig

	// WithCompilationCache configures how runtime caches the compiled modules. In the default configuration, compilation results are
	// only in-memory until Runtime.Close is closed, and not shareable by multiple Runtime.
	//
	// Below defines the shared cache across multiple instances of Runtime:
	//
	//	// Creates the new Cache and the runtime configuration with it.
	//	cache := wazero.NewCompilationCache()
	//	defer cache.Close()
	//	config := wazero.NewRuntimeConfig().WithCompilationCache(c)
	//
	//	// Creates two runtimes while sharing compilation caches.
	//	foo := wazero.NewRuntimeWithConfig(context.Background(), config)
	// 	bar := wazero.NewRuntimeWithConfig(context.Background(), config)
	//
	// # Cache Key
	//
	// Cached files are keyed on the version of wazero. This is obtained from go.mod of your application,
	// and we use it to verify the compatibility of caches against the currently-running wazero.
	// However, if you use this in tests of a package not named as `main`, then wazero cannot obtain the correct
	// version of wazero due to the known issue of debug.BuildInfo function: https://github.com/golang/go/issues/33976.
	// As a consequence, your cache won't contain the correct version information and always be treated as `dev` version.
	// To avoid this issue, you can pass -ldflags "-X github.com/tetratelabs/wazero/internal/version.version=foo" when running tests.
	WithCompilationCache(CompilationCache) RuntimeConfig

	// WithCustomSections toggles parsing of "custom sections". Defaults to false.
	//
	// When enabled, it is possible to retrieve custom sections from a CompiledModule:
	//
	//	config := wazero.NewRuntimeConfig().WithCustomSections(true)
	//	r := wazero.NewRuntimeWithConfig(ctx, config)
	//	c, err := r.CompileModule(ctx, wasm)
	//	customSections := c.CustomSections()
	WithCustomSections(bool) RuntimeConfig

	// WithCloseOnContextDone ensures the executions of functions to be terminated under one of the following circumstances:
	//
	// 	- context.Context passed to the Call method of api.Function is canceled during execution. (i.e. ctx by context.WithCancel)
	// 	- context.Context passed to the Call method of api.Function reaches timeout during execution. (i.e. ctx by context.WithTimeout or context.WithDeadline)
	// 	- Close or CloseWithExitCode of api.Module is explicitly called during execution.
	//
	// This is especially useful when one wants to run untrusted Wasm binaries since otherwise, any invocation of
	// api.Function can potentially block the corresponding Goroutine forever. Moreover, it might block the
	// entire underlying OS thread which runs the api.Function call. See "Why it's safe to execute runtime-generated
	// machine codes against async Goroutine preemption" section in RATIONALE.md for detail.
	//
	// Upon the termination of the function executions, api.Module is closed.
	//
	// Note that this comes with a bit of extra cost when enabled. The reason is that internally this forces
	// interpreter and compiler runtimes to insert the periodical checks on the conditions above. For that reason,
	// this is disabled by default.
	//
	// See examples in context_done_example_test.go for the end-to-end demonstrations.
	//
	// When the invocations of api.Function are closed due to this, sys.ExitError is raised to the callers and
	// the api.Module from which the functions are derived is made closed.
	WithCloseOnContextDone(bool) RuntimeConfig
}

// NewRuntimeConfig returns a RuntimeConfig using the compiler if it is supported in this environment,
// or the interpreter otherwise.
func NewRuntimeConfig() RuntimeConfig {
	ret := engineLessConfig.clone()
	ret.engineKind = engineKindAuto
	return ret
}

type newEngine func(context.Context, api.CoreFeatures, filecache.Cache) wasm.Engine

type runtimeConfig struct {
	enabledFeatures       api.CoreFeatures
	memoryLimitPages      uint32
	memoryCapacityFromMax bool
	engineKind            engineKind
	dwarfDisabled         bool // negative as defaults to enabled
	newEngine             newEngine
	cache                 CompilationCache
	storeCustomSections   bool
	ensureTermination     bool
}

// engineLessConfig helps avoid copy/pasting the wrong defaults.
var engineLessConfig = &runtimeConfig{
	enabledFeatures:       api.CoreFeaturesV2,
	memoryLimitPages:      wasm.MemoryLimitPages,
	memoryCapacityFromMax: false,
	dwarfDisabled:         false,
}

type engineKind int

const (
	engineKindAuto engineKind = iota - 1
	engineKindCompiler
	engineKindInterpreter
	engineKindCount
)

// NewRuntimeConfigCompiler compiles WebAssembly modules into
// runtime.GOARCH-specific assembly for optimal performance.
//
// The default implementation is AOT (Ahead of Time) compilation, applied at
// Runtime.CompileModule. This allows consistent runtime performance, as well
// the ability to reduce any first request penalty.
//
// Note: While this is technically AOT, this does not imply any action on your
// part. wazero automatically performs ahead-of-time compilation as needed when
// Runtime.CompileModule is invoked.
//
// # Warning
//
//   - This panics at runtime if the runtime.GOOS or runtime.GOARCH does not
//     support compiler. Use NewRuntimeConfig to safely detect and fallback to
//     NewRuntimeConfigInterpreter if needed.
//
//   - If you are using wazero in buildmode=c-archive or c-shared, make sure that you set up the alternate signal stack
//     by using, e.g. `sigaltstack` combined with `SA_ONSTACK` flag on `sigaction` on Linux,
//     before calling any api.Function. This is because the Go runtime does not set up the alternate signal stack
//     for c-archive or c-shared modes, and wazero uses the different stack than the calling Goroutine.
//     Hence, the signal handler might get invoked on the wazero's stack, which may cause a stack overflow.
//     https://github.com/tetratelabs/wazero/blob/2092c0a879f30d49d7b37f333f4547574b8afe0d/internal/integration_test/fuzz/fuzz/tests/sigstack.rs#L19-L36
func NewRuntimeConfigCompiler() RuntimeConfig {
	ret := engineLessConfig.clone()
	ret.engineKind = engineKindCompiler
	return ret
}

// NewRuntimeConfigInterpreter interprets WebAssembly modules instead of compiling them into assembly.
func NewRuntimeConfigInterpreter() RuntimeConfig {
	ret := engineLessConfig.clone()
	ret.engineKind = engineKindInterpreter
	return ret
}

// clone makes a deep copy of this runtime config.
func (c *runtimeConfig) clone() *runtimeConfig {
	ret := *c // copy except maps which share a ref
	return &ret
}

// WithCoreFeatures implements RuntimeConfig.WithCoreFeatures
func (c *runtimeConfig) WithCoreFeatures(features api.CoreFeatures) RuntimeConfig {
	ret := c.clone()
	ret.enabledFeatures = features
	return ret
}

// WithCloseOnContextDone implements RuntimeConfig.WithCloseOnContextDone
func (c *runtimeConfig) WithCloseOnContextDone(ensure bool) RuntimeConfig {
	ret := c.clone()
	ret.ensureTermination = ensure
	return ret
}

// WithMemoryLimitPages implements RuntimeConfig.WithMemoryLimitPages
func (c *runtimeConfig) WithMemoryLimitPages(memoryLimitPages uint32) RuntimeConfig {
	ret := c.clone()
	// This panics instead of returning an error as it is unlikely.
	if memoryLimitPages > wasm.MemoryLimitPages {
		panic(fmt.Errorf("memoryLimitPages invalid: %d > %d", memoryLimitPages, wasm.MemoryLimitPages))
	}
	ret.memoryLimitPages = memoryLimitPages
	return ret
}

// WithCompilationCache implements RuntimeConfig.WithCompilationCache
func (c *runtimeConfig) WithCompilationCache(ca CompilationCache) RuntimeConfig {
	ret := c.clone()
	ret.cache = ca
	return ret
}

// WithMemoryCapacityFromMax implements RuntimeConfig.WithMemoryCapacityFromMax
func (c *runtimeConfig) WithMemoryCapacityFromMax(memoryCapacityFromMax bool) RuntimeConfig {
	ret := c.clone()
	ret.memoryCapacityFromMax = memoryCapacityFromMax
	return ret
}

// WithDebugInfoEnabled implements RuntimeConfig.WithDebugInfoEnabled
func (c *runtimeConfig) WithDebugInfoEnabled(dwarfEnabled bool) RuntimeConfig {
	ret := c.clone()
	ret.dwarfDisabled = !dwarfEnabled
	return ret
}

// WithCustomSections implements RuntimeConfig.WithCustomSections
func (c *runtimeConfig) WithCustomSections(storeCustomSections bool) RuntimeConfig {
	ret := c.clone()
	ret.storeCustomSections = storeCustomSections
	return ret
}

// CompiledModule is a WebAssembly module ready to be instantiated (Runtime.InstantiateModule) as an api.Module.
//
// In WebAssembly terminology, this is a decoded, validated, and possibly also compiled module. wazero avoids using
// the name "Module" for both before and after instantiation as the name conflation has caused confusion.
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/#semantic-phases%E2%91%A0
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - Closing the wazero.Runtime closes any CompiledModule it compiled.
type CompiledModule interface {
	// Name returns the module name encoded into the binary or empty if not.
	Name() string

	// ImportedFunctions returns all the imported functions
	// (api.FunctionDefinition) in this module or nil if there are none.
	//
	// Note: Unlike ExportedFunctions, there is no unique constraint on
	// imports.
	ImportedFunctions() []api.FunctionDefinition

	// ExportedFunctions returns all the exported functions
	// (api.FunctionDefinition) in this module keyed on export name.
	ExportedFunctions() map[string]api.FunctionDefinition

	// ImportedMemories returns all the imported memories
	// (api.MemoryDefinition) in this module or nil if there are none.
	//
	// ## Notes
	//   - As of WebAssembly Core Specification 2.0, there can be at most one
	//     memory.
	//   - Unlike ExportedMemories, there is no unique constraint on imports.
	ImportedMemories() []api.MemoryDefinition

	// ExportedMemories returns all the exported memories
	// (api.MemoryDefinition) in this module keyed on export name.
	//
	// Note: As of WebAssembly Core Specification 2.0, there can be at most one
	// memory.
	ExportedMemories() map[string]api.MemoryDefinition

	// CustomSections returns all the custom sections
	// (api.CustomSection) in this module keyed on the section name.
	CustomSections() []api.CustomSection

	// Close releases all the allocated resources for this CompiledModule.
	//
	// Note: It is safe to call Close while having outstanding calls from an
	// api.Module instantiated from this.
	Close(context.Context) error
}

// compile-time check to ensure compiledModule implements CompiledModule
var _ CompiledModule = &compiledModule{}

type compiledModule struct {
	module *wasm.Module
	// compiledEngine holds an engine on which `module` is compiled.
	compiledEngine wasm.Engine
	// closeWithModule prevents leaking compiled code when a module is compiled implicitly.
	closeWithModule bool
	typeIDs         []wasm.FunctionTypeID
}

// Name implements CompiledModule.Name
func (c *compiledModule) Name() (moduleName string) {
	if ns := c.module.NameSection; ns != nil {
		moduleName = ns.ModuleName
	}
	return
}

// Close implements CompiledModule.Close
func (c *compiledModule) Close(context.Context) error {
	c.compiledEngine.DeleteCompiledModule(c.module)
	// It is possible the underlying may need to return an error later, but in any case this matches api.Module.Close.
	return nil
}

// ImportedFunctions implements CompiledModule.ImportedFunctions
func (c *compiledModule) ImportedFunctions() []api.FunctionDefinition {
	return c.module.ImportedFunctions()
}

// ExportedFunctions implements CompiledModule.ExportedFunctions
func (c *compiledModule) ExportedFunctions() map[string]api.FunctionDefinition {
	return c.module.ExportedFunctions()
}

// ImportedMemories implements CompiledModule.ImportedMemories
func (c *compiledModule) ImportedMemories() []api.MemoryDefinition {
	return c.module.ImportedMemories()
}

// ExportedMemories implements CompiledModule.ExportedMemories
func (c *compiledModule) ExportedMemories() map[string]api.MemoryDefinition {
	return c.module.ExportedMemories()
}

// CustomSections implements CompiledModule.CustomSections
func (c *compiledModule) CustomSections() []api.CustomSection {
	ret := make([]api.CustomSection, len(c.module.CustomSections))
	for i, d := range c.module.CustomSections {
		ret[i] = &customSection{data: d.Data, name: d.Name}
	}
	return ret
}

// customSection implements wasm.CustomSection
type customSection struct {
	internalapi.WazeroOnlyType
	name string
	data []byte
}

// Name implements wasm.CustomSection.Name
func (c *customSection) Name() string {
	return c.name
}

// Data implements wasm.CustomSection.Data
func (c *customSection) Data() []byte {
	return c.data
}

// ModuleConfig configures resources needed by functions that have low-level interactions with the host operating
// system. Using this, resources such as STDIN can be isolated, so that the same module can be safely instantiated
// multiple times.
//
// Here's an example:
//
//	// Initialize base configuration:
//	config := wazero.NewModuleConfig().WithStdout(buf).WithSysNanotime()
//
//	// Assign different configuration on each instantiation
//	mod, _ := r.InstantiateModule(ctx, compiled, config.WithName("rotate").WithArgs("rotate", "angle=90", "dir=cw"))
//
// While wazero supports Windows as a platform, host functions using ModuleConfig follow a UNIX dialect.
// See RATIONALE.md for design background and relationship to WebAssembly System Interfaces (WASI).
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
//   - ModuleConfig is immutable. Each WithXXX function returns a new instance
//     including the corresponding change.
type ModuleConfig interface {
	// WithArgs assigns command-line arguments visible to an imported function that reads an arg vector (argv). Defaults to
	// none. Runtime.InstantiateModule errs if any arg is empty.
	//
	// These values are commonly read by the functions like "args_get" in "wasi_snapshot_preview1" although they could be
	// read by functions imported from other modules.
	//
	// Similar to os.Args and exec.Cmd Env, many implementations would expect a program name to be argv[0]. However, neither
	// WebAssembly nor WebAssembly System Interfaces (WASI) define this. Regardless, you may choose to set the first
	// argument to the same value set via WithName.
	//
	// Note: This does not default to os.Args as that violates sandboxing.
	//
	// See https://linux.die.net/man/3/argv and https://en.wikipedia.org/wiki/Null-terminated_string
	WithArgs(...string) ModuleConfig

	// WithEnv sets an environment variable visible to a Module that imports functions. Defaults to none.
	// Runtime.InstantiateModule errs if the key is empty or contains a NULL(0) or equals("") character.
	//
	// Validation is the same as os.Setenv on Linux and replaces any existing value. Unlike exec.Cmd Env, this does not
	// default to the current process environment as that would violate sandboxing. This also does not preserve order.
	//
	// Environment variables are commonly read by the functions like "environ_get" in "wasi_snapshot_preview1" although
	// they could be read by functions imported from other modules.
	//
	// While similar to process configuration, there are no assumptions that can be made about anything OS-specific. For
	// example, neither WebAssembly nor WebAssembly System Interfaces (WASI) define concerns processes have, such as
	// case-sensitivity on environment keys. For portability, define entries with case-insensitively unique keys.
	//
	// See https://linux.die.net/man/3/environ and https://en.wikipedia.org/wiki/Null-terminated_string
	WithEnv(key, value string) ModuleConfig

	// WithFS is a convenience that calls WithFSConfig with an FSConfig of the
	// input for the root ("/") guest path.
	WithFS(fs.FS) ModuleConfig

	// WithFSConfig configures the filesystem available to each guest
	// instantiated with this configuration. By default, no file access is
	// allowed, so functions like `path_open` result in unsupported errors
	// (e.g. syscall.ENOSYS).
	WithFSConfig(FSConfig) ModuleConfig

	// WithName configures the module name. Defaults to what was decoded from
	// the name section. Duplicate names are not allowed in a single Runtime.
	//
	// Calling this with the empty string "" makes the module anonymous.
	// That is useful when you want to instantiate the same CompiledModule multiple times like below:
	//
	// 	for i := 0; i < N; i++ {
	//		// Instantiate a new Wasm module from the already compiled `compiledWasm` anonymously without a name.
	//		instance, err := r.InstantiateModule(ctx, compiledWasm, wazero.NewModuleConfig().WithName(""))
	//		// ....
	//	}
	//
	// See the `concurrent-instantiation` example for a complete usage.
	//
	// Non-empty named modules are available for other modules to import by name.
	WithName(string) ModuleConfig

	// WithStartFunctions configures the functions to call after the module is
	// instantiated. Defaults to "_start".
	//
	// Clearing the default is supported, via `WithStartFunctions()`.
	//
	// # Notes
	//
	//   - If a start function doesn't exist, it is skipped. However, any that
	//     do exist are called in order.
	//   - Start functions are not intended to be called multiple times.
	//     Functions that should be called multiple times should be invoked
	//     manually via api.Module's `ExportedFunction` method.
	//   - Start functions commonly exit the module during instantiation,
	//     preventing use of any functions later. This is the case in "wasip1",
	//     which defines the default value "_start".
	//   - See /RATIONALE.md for motivation of this feature.
	WithStartFunctions(...string) ModuleConfig

	// WithStderr configures where standard error (file descriptor 2) is written. Defaults to io.Discard.
	//
	// This writer is most commonly used by the functions like "fd_write" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// # Notes
	//
	//   - The caller is responsible to close any io.Writer they supply: It is not closed on api.Module Close.
	//   - This does not default to os.Stderr as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stderr
	WithStderr(io.Writer) ModuleConfig

	// WithStdin configures where standard input (file descriptor 0) is read. Defaults to return io.EOF.
	//
	// This reader is most commonly used by the functions like "fd_read" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// # Notes
	//
	//   - The caller is responsible to close any io.Reader they supply: It is not closed on api.Module Close.
	//   - This does not default to os.Stdin as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stdin
	WithStdin(io.Reader) ModuleConfig

	// WithStdout configures where standard output (file descriptor 1) is written. Defaults to io.Discard.
	//
	// This writer is most commonly used by the functions like "fd_write" in "wasi_snapshot_preview1" although it could
	// be used by functions imported from other modules.
	//
	// # Notes
	//
	//   - The caller is responsible to close any io.Writer they supply: It is not closed on api.Module Close.
	//   - This does not default to os.Stdout as that both violates sandboxing and prevents concurrent modules.
	//
	// See https://linux.die.net/man/3/stdout
	WithStdout(io.Writer) ModuleConfig

	// WithWalltime configures the wall clock, sometimes referred to as the
	// real time clock. sys.Walltime returns the current unix/epoch time,
	// seconds since midnight UTC 1 January 1970, with a nanosecond fraction.
	// This defaults to a fake result that increases by 1ms on each reading.
	//
	// Here's an example that uses a custom clock:
	//	moduleConfig = moduleConfig.
	//		WithWalltime(func(context.Context) (sec int64, nsec int32) {
	//			return clock.walltime()
	//		}, sys.ClockResolution(time.Microsecond.Nanoseconds()))
	//
	// # Notes:
	//   - This does not default to time.Now as that violates sandboxing.
	//   - This is used to implement host functions such as WASI
	//     `clock_time_get` with the `realtime` clock ID.
	//   - Use WithSysWalltime for a usable implementation.
	WithWalltime(sys.Walltime, sys.ClockResolution) ModuleConfig

	// WithSysWalltime uses time.Now for sys.Walltime with a resolution of 1us
	// (1000ns).
	//
	// See WithWalltime
	WithSysWalltime() ModuleConfig

	// WithNanotime configures the monotonic clock, used to measure elapsed
	// time in nanoseconds. Defaults to a fake result that increases by 1ms
	// on each reading.
	//
	// Here's an example that uses a custom clock:
	//	moduleConfig = moduleConfig.
	//		WithNanotime(func(context.Context) int64 {
	//			return clock.nanotime()
	//		}, sys.ClockResolution(time.Microsecond.Nanoseconds()))
	//
	// # Notes:
	//   - This does not default to time.Since as that violates sandboxing.
	//   - This is used to implement host functions such as WASI
	//     `clock_time_get` with the `monotonic` clock ID.
	//   - Some compilers implement sleep by looping on sys.Nanotime (e.g. Go).
	//   - If you set this, you should probably set WithNanosleep also.
	//   - Use WithSysNanotime for a usable implementation.
	WithNanotime(sys.Nanotime, sys.ClockResolution) ModuleConfig

	// WithSysNanotime uses time.Now for sys.Nanotime with a resolution of 1us.
	//
	// See WithNanotime
	WithSysNanotime() ModuleConfig

	// WithNanosleep configures the how to pause the current goroutine for at
	// least the configured nanoseconds. Defaults to return immediately.
	//
	// This example uses a custom sleep function:
	//	moduleConfig = moduleConfig.
	//		WithNanosleep(func(ns int64) {
	//			rel := unix.NsecToTimespec(ns)
	//			remain := unix.Timespec{}
	//			for { // loop until no more time remaining
	//				err := unix.ClockNanosleep(unix.CLOCK_MONOTONIC, 0, &rel, &remain)
	//			--snip--
	//
	// # Notes:
	//   - This does not default to time.Sleep as that violates sandboxing.
	//   - This is used to implement host functions such as WASI `poll_oneoff`.
	//   - Some compilers implement sleep by looping on sys.Nanotime (e.g. Go).
	//   - If you set this, you should probably set WithNanotime also.
	//   - Use WithSysNanosleep for a usable implementation.
	WithNanosleep(sys.Nanosleep) ModuleConfig

	// WithOsyield yields the processor, typically to implement spin-wait
	// loops. Defaults to return immediately.
	//
	// # Notes:
	//   - This primarily supports `sched_yield` in WASI
	//   - This does not default to runtime.osyield as that violates sandboxing.
	WithOsyield(sys.Osyield) ModuleConfig

	// WithSysNanosleep uses time.Sleep for sys.Nanosleep.
	//
	// See WithNanosleep
	WithSysNanosleep() ModuleConfig

	// WithRandSource configures a source of random bytes. Defaults to return a
	// deterministic source. You might override this with crypto/rand.Reader
	//
	// This reader is most commonly used by the functions like "random_get" in
	// "wasi_snapshot_preview1", "seed" in AssemblyScript standard "env", and
	// "getRandomData" when runtime.GOOS is "js".
	//
	// Note: The caller is responsible to close any io.Reader they supply: It
	// is not closed on api.Module Close.
	WithRandSource(io.Reader) ModuleConfig
}

type moduleConfig struct {
	name               string
	nameSet            bool
	startFunctions     []string
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	randSource         io.Reader
	walltime           sys.Walltime
	walltimeResolution sys.ClockResolution
	nanotime           sys.Nanotime
	nanotimeResolution sys.ClockResolution
	nanosleep          sys.Nanosleep
	osyield            sys.Osyield
	args               [][]byte
	// environ is pair-indexed to retain order similar to os.Environ.
	environ [][]byte
	// environKeys allow overwriting of existing values.
	environKeys map[string]int
	// fsConfig is the file system configuration for ABI like WASI.
	fsConfig FSConfig
	// sockConfig is the network listener configuration for ABI like WASI.
	sockConfig *internalsock.Config
}

// NewModuleConfig returns a ModuleConfig that can be used for configuring module instantiation.
func NewModuleConfig() ModuleConfig {
	return &moduleConfig{
		startFunctions: []string{"_start"},
		environKeys:    map[string]int{},
	}
}

// clone makes a deep copy of this module config.
func (c *moduleConfig) clone() *moduleConfig {
	ret := *c // copy except maps which share a ref
	ret.environKeys = make(map[string]int, len(c.environKeys))
	for key, value := range c.environKeys {
		ret.environKeys[key] = value
	}
	return &ret
}

// WithArgs implements ModuleConfig.WithArgs
func (c *moduleConfig) WithArgs(args ...string) ModuleConfig {
	ret := c.clone()
	ret.args = toByteSlices(args)
	return ret
}

func toByteSlices(strings []string) (result [][]byte) {
	if len(strings) == 0 {
		return
	}
	result = make([][]byte, len(strings))
	for i, a := range strings {
		result[i] = []byte(a)
	}
	return
}

// WithEnv implements ModuleConfig.WithEnv
func (c *moduleConfig) WithEnv(key, value string) ModuleConfig {
	ret := c.clone()
	// Check to see if this key already exists and update it.
	if i, ok := ret.environKeys[key]; ok {
		ret.environ[i+1] = []byte(value) // environ is pair-indexed, so the value is 1 after the key.
	} else {
		ret.environKeys[key] = len(ret.environ)
		ret.environ = append(ret.environ, []byte(key), []byte(value))
	}
	return ret
}

// WithFS implements ModuleConfig.WithFS
func (c *moduleConfig) WithFS(fs fs.FS) ModuleConfig {
	var config FSConfig
	if fs != nil {
		config = NewFSConfig().WithFSMount(fs, "")
	}
	return c.WithFSConfig(config)
}

// WithFSConfig implements ModuleConfig.WithFSConfig
func (c *moduleConfig) WithFSConfig(config FSConfig) ModuleConfig {
	ret := c.clone()
	ret.fsConfig = config
	return ret
}

// WithName implements ModuleConfig.WithName
func (c *moduleConfig) WithName(name string) ModuleConfig {
	ret := c.clone()
	ret.nameSet = true
	ret.name = name
	return ret
}

// WithStartFunctions implements ModuleConfig.WithStartFunctions
func (c *moduleConfig) WithStartFunctions(startFunctions ...string) ModuleConfig {
	ret := c.clone()
	ret.startFunctions = startFunctions
	return ret
}

// WithStderr implements ModuleConfig.WithStderr
func (c *moduleConfig) WithStderr(stderr io.Writer) ModuleConfig {
	ret := c.clone()
	ret.stderr = stderr
	return ret
}

// WithStdin implements ModuleConfig.WithStdin
func (c *moduleConfig) WithStdin(stdin io.Reader) ModuleConfig {
	ret := c.clone()
	ret.stdin = stdin
	return ret
}

// WithStdout implements ModuleConfig.WithStdout
func (c *moduleConfig) WithStdout(stdout io.Writer) ModuleConfig {
	ret := c.clone()
	ret.stdout = stdout
	return ret
}

// WithWalltime implements ModuleConfig.WithWalltime
func (c *moduleConfig) WithWalltime(walltime sys.Walltime, resolution sys.ClockResolution) ModuleConfig {
	ret := c.clone()
	ret.walltime = walltime
	ret.walltimeResolution = resolution
	return ret
}

// We choose arbitrary resolutions here because there's no perfect alternative. For example, according to the
// source in time.go, windows monotonic resolution can be 15ms. This chooses arbitrarily 1us for wall time and
// 1ns for monotonic. See RATIONALE.md for more context.

// WithSysWalltime implements ModuleConfig.WithSysWalltime
func (c *moduleConfig) WithSysWalltime() ModuleConfig {
	return c.WithWalltime(platform.Walltime, sys.ClockResolution(time.Microsecond.Nanoseconds()))
}

// WithNanotime implements ModuleConfig.WithNanotime
func (c *moduleConfig) WithNanotime(nanotime sys.Nanotime, resolution sys.ClockResolution) ModuleConfig {
	ret := c.clone()
	ret.nanotime = nanotime
	ret.nanotimeResolution = resolution
	return ret
}

// WithSysNanotime implements ModuleConfig.WithSysNanotime
func (c *moduleConfig) WithSysNanotime() ModuleConfig {
	return c.WithNanotime(platform.Nanotime, sys.ClockResolution(1))
}

// WithNanosleep implements ModuleConfig.WithNanosleep
func (c *moduleConfig) WithNanosleep(nanosleep sys.Nanosleep) ModuleConfig {
	ret := *c // copy
	ret.nanosleep = nanosleep
	return &ret
}

// WithOsyield implements ModuleConfig.WithOsyield
func (c *moduleConfig) WithOsyield(osyield sys.Osyield) ModuleConfig {
	ret := *c // copy
	ret.osyield = osyield
	return &ret
}

// WithSysNanosleep implements ModuleConfig.WithSysNanosleep
func (c *moduleConfig) WithSysNanosleep() ModuleConfig {
	return c.WithNanosleep(platform.Nanosleep)
}

// WithRandSource implements ModuleConfig.WithRandSource
func (c *moduleConfig) WithRandSource(source io.Reader) ModuleConfig {
	ret := c.clone()
	ret.randSource = source
	return ret
}

// toSysContext creates a baseline wasm.Context configured by ModuleConfig.
func (c *moduleConfig) toSysContext() (sysCtx *internalsys.Context, err error) {
	var environ [][]byte // Intentionally doesn't pre-allocate to reduce logic to default to nil.
	// Same validation as syscall.Setenv for Linux
	for i := 0; i < len(c.environ); i += 2 {
		key, value := c.environ[i], c.environ[i+1]
		keyLen := len(key)
		if keyLen == 0 {
			err = errors.New("environ invalid: empty key")
			return
		}
		valueLen := len(value)
		result := make([]byte, keyLen+valueLen+1)
		j := 0
		for ; j < keyLen; j++ {
			if k := key[j]; k == '=' { // NUL enforced in NewContext
				err = errors.New("environ invalid: key contains '=' character")
				return
			} else {
				result[j] = k
			}
		}
		result[j] = '='
		copy(result[j+1:], value)
		environ = append(environ, result)
	}

	var fs []experimentalsys.FS
	var guestPaths []string
	if f, ok := c.fsConfig.(*fsConfig); ok {
		fs, guestPaths = f.preopens()
	}

	var listeners []*net.TCPListener
	if n := c.sockConfig; n != nil {
		if listeners, err = n.BuildTCPListeners(); err != nil {
			return
		}
	}

	return internalsys.NewContext(
		math.MaxUint32,
		c.args,
		environ,
		c.stdin,
		c.stdout,
		c.stderr,
		c.randSource,
		c.walltime, c.walltimeResolution,
		c.nanotime, c.nanotimeResolution,
		c.nanosleep, c.osyield,
		fs, guestPaths,
		listeners,
	)
}
