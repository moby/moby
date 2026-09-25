package sys

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/sys"
)

// Context holds module-scoped system resources currently only supported by
// built-in host functions.
type Context struct {
	args, environ         [][]byte
	argsSize, environSize uint32

	walltime           sys.Walltime
	walltimeResolution sys.ClockResolution
	nanotime           sys.Nanotime
	nanotimeResolution sys.ClockResolution
	nanosleep          sys.Nanosleep
	osyield            sys.Osyield
	randSource         io.Reader
	fsc                FSContext
}

// Args is like os.Args and defaults to nil.
//
// Note: The count will never be more than math.MaxUint32.
// See wazero.ModuleConfig WithArgs
func (c *Context) Args() [][]byte {
	return c.args
}

// ArgsSize is the size to encode Args as Null-terminated strings.
//
// Note: To get the size without null-terminators, subtract the length of Args from this value.
// See wazero.ModuleConfig WithArgs
// See https://en.wikipedia.org/wiki/Null-terminated_string
func (c *Context) ArgsSize() uint32 {
	return c.argsSize
}

// Environ are "key=value" entries like os.Environ and default to nil.
//
// Note: The count will never be more than math.MaxUint32.
// See wazero.ModuleConfig WithEnv
func (c *Context) Environ() [][]byte {
	return c.environ
}

// EnvironSize is the size to encode Environ as Null-terminated strings.
//
// Note: To get the size without null-terminators, subtract the length of Environ from this value.
// See wazero.ModuleConfig WithEnv
// See https://en.wikipedia.org/wiki/Null-terminated_string
func (c *Context) EnvironSize() uint32 {
	return c.environSize
}

// Walltime implements platform.Walltime.
func (c *Context) Walltime() (sec int64, nsec int32) {
	return c.walltime()
}

// WalltimeNanos returns platform.Walltime as epoch nanoseconds.
func (c *Context) WalltimeNanos() int64 {
	sec, nsec := c.Walltime()
	return (sec * time.Second.Nanoseconds()) + int64(nsec)
}

// WalltimeResolution returns resolution of Walltime.
func (c *Context) WalltimeResolution() sys.ClockResolution {
	return c.walltimeResolution
}

// Nanotime implements sys.Nanotime.
func (c *Context) Nanotime() int64 {
	return c.nanotime()
}

// NanotimeResolution returns resolution of Nanotime.
func (c *Context) NanotimeResolution() sys.ClockResolution {
	return c.nanotimeResolution
}

// Nanosleep implements sys.Nanosleep.
func (c *Context) Nanosleep(ns int64) {
	c.nanosleep(ns)
}

// Osyield implements sys.Osyield.
func (c *Context) Osyield() {
	c.osyield()
}

// FS returns the possibly empty (UnimplementedFS) file system context.
func (c *Context) FS() *FSContext {
	return &c.fsc
}

// RandSource is a source of random bytes and defaults to a deterministic source.
// see wazero.ModuleConfig WithRandSource
func (c *Context) RandSource() io.Reader {
	return c.randSource
}

// DefaultContext returns Context with no values set except a possible nil
// sys.FS.
//
// Note: This is only used for testing.
func DefaultContext(fs experimentalsys.FS) *Context {
	if sysCtx, err := NewContext(0, nil, nil, nil, nil, nil, nil, nil, 0, nil, 0, nil, nil, []experimentalsys.FS{fs}, []string{""}, nil); err != nil {
		panic(fmt.Errorf("BUG: DefaultContext should never error: %w", err))
	} else {
		return sysCtx
	}
}

// NewContext is a factory function which helps avoid needing to know defaults or exporting all fields.
// Note: max is exposed for testing. max is only used for env/args validation.
func NewContext(
	max uint32,
	args, environ [][]byte,
	stdin io.Reader,
	stdout, stderr io.Writer,
	randSource io.Reader,
	walltime sys.Walltime,
	walltimeResolution sys.ClockResolution,
	nanotime sys.Nanotime,
	nanotimeResolution sys.ClockResolution,
	nanosleep sys.Nanosleep,
	osyield sys.Osyield,
	fs []experimentalsys.FS, guestPaths []string,
	tcpListeners []*net.TCPListener,
) (sysCtx *Context, err error) {
	sysCtx = &Context{args: args, environ: environ}

	if sysCtx.argsSize, err = nullTerminatedByteCount(max, args); err != nil {
		return nil, fmt.Errorf("args invalid: %w", err)
	}

	if sysCtx.environSize, err = nullTerminatedByteCount(max, environ); err != nil {
		return nil, fmt.Errorf("environ invalid: %w", err)
	}

	if randSource == nil {
		sysCtx.randSource = platform.NewFakeRandSource()
	} else {
		sysCtx.randSource = randSource
	}

	if walltime != nil {
		if clockResolutionInvalid(walltimeResolution) {
			return nil, fmt.Errorf("invalid Walltime resolution: %d", walltimeResolution)
		}
		sysCtx.walltime = walltime
		sysCtx.walltimeResolution = walltimeResolution
	} else {
		sysCtx.walltime = platform.NewFakeWalltime()
		sysCtx.walltimeResolution = sys.ClockResolution(time.Microsecond.Nanoseconds())
	}

	if nanotime != nil {
		if clockResolutionInvalid(nanotimeResolution) {
			return nil, fmt.Errorf("invalid Nanotime resolution: %d", nanotimeResolution)
		}
		sysCtx.nanotime = nanotime
		sysCtx.nanotimeResolution = nanotimeResolution
	} else {
		sysCtx.nanotime = platform.NewFakeNanotime()
		sysCtx.nanotimeResolution = sys.ClockResolution(time.Nanosecond)
	}

	if nanosleep != nil {
		sysCtx.nanosleep = nanosleep
	} else {
		sysCtx.nanosleep = platform.FakeNanosleep
	}

	if osyield != nil {
		sysCtx.osyield = osyield
	} else {
		sysCtx.osyield = platform.FakeOsyield
	}

	err = sysCtx.InitFSContext(stdin, stdout, stderr, fs, guestPaths, tcpListeners)

	return
}

// clockResolutionInvalid returns true if the value stored isn't reasonable.
func clockResolutionInvalid(resolution sys.ClockResolution) bool {
	return resolution < 1 || resolution > sys.ClockResolution(time.Hour.Nanoseconds())
}

// nullTerminatedByteCount ensures the count or Nul-terminated length of the elements doesn't exceed max, and that no
// element includes the nul character.
func nullTerminatedByteCount(max uint32, elements [][]byte) (uint32, error) {
	count := uint32(len(elements))
	if count > max {
		return 0, errors.New("exceeds maximum count")
	}

	// The buffer size is the total size including null terminators. The null terminator count == value count, sum
	// count with each value length. This works because in Go, the length of a string is the same as its byte count.
	bufSize, maxSize := uint64(count), uint64(max) // uint64 to allow summing without overflow
	for _, e := range elements {
		// As this is null-terminated, We have to validate there are no null characters in the string.
		for _, c := range e {
			if c == 0 {
				return 0, errors.New("contains NUL character")
			}
		}

		nextSize := bufSize + uint64(len(e))
		if nextSize > maxSize {
			return 0, errors.New("exceeds maximum size")
		}
		bufSize = nextSize

	}
	return uint32(bufSize), nil
}
