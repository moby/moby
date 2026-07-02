//go:build windows

package runhcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	irunhcs "github.com/Microsoft/hcsshim/internal/runhcs"
	"github.com/containerd/go-runc"
)

// Format is the type of log formatting options available.
type Format string

const (
	none Format = ""
	// Text is the default text log output.
	Text Format = "text"
	// JSON is the JSON formatted log output.
	JSON Format = "json"
)

var runhcsPath atomic.Value

func getCommandPath() string {
	const command = "runhcs.exe"

	pathi := runhcsPath.Load()
	if pathi == nil {
		path, err := exec.LookPath(command)
		if err != nil {
			if errors.Is(err, exec.ErrDot) {
				err = nil
			}
		}
		if err != nil {
			// LookPath only finds current directory matches based on the
			// callers current directory but the caller is not likely in the
			// same directory as the containerd executables. Instead match the
			// calling binaries path (a containerd shim usually) and see if they
			// are side by side. If so execute the runhcs.exe found there.
			if self, serr := os.Executable(); serr == nil {
				testPath := filepath.Join(filepath.Dir(self), command)
				if _, serr := os.Stat(testPath); serr == nil {
					path = testPath
				}
			}
			if path == "" {
				// Failed to look up command just use it directly and let the
				// Windows loader find it.
				path = command
			}
			runhcsPath.Store(path)
			return path
		}
		apath, err := filepath.Abs(path)
		if err != nil {
			// We couldnt make `path` an `AbsPath`. Just use `path` directly and
			// let the Windows loader find it.
			apath = path
		}
		runhcsPath.Store(apath)
		return apath
	}
	return pathi.(string)
}

var bytesBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(nil)
	},
}

func getBuf() *bytes.Buffer {
	return bytesBufferPool.Get().(*bytes.Buffer)
}

func putBuf(b *bytes.Buffer) {
	b.Reset()
	bytesBufferPool.Put(b)
}

// Runhcs is the client to the runhcs cli.
type Runhcs struct {
	// Debug enables debug output for logging.
	Debug bool
	// Log sets the log file path or named pipe (e.g. \\.\pipe\ProtectedPrefix\Administrators\runhcs-log) where internal debug information is written.
	Log string
	// LogFormat sets the format used by logs.
	LogFormat Format
	// Owner sets the compute system owner property.
	Owner string
	// Root is the registry key root for storage of runhcs container state.
	Root string
}

func (r *Runhcs) args() []string {
	var out []string
	if r.Debug {
		out = append(out, "--debug")
	}
	if r.Log != "" {
		if strings.HasPrefix(r.Log, irunhcs.SafePipePrefix) {
			out = append(out, "--log", r.Log)
		} else {
			abs, err := filepath.Abs(r.Log)
			if err == nil {
				out = append(out, "--log", abs)
			}
		}
	}
	if r.LogFormat != none {
		out = append(out, "--log-format", string(r.LogFormat))
	}
	if r.Owner != "" {
		out = append(out, "--owner", r.Owner)
	}
	if r.Root != "" {
		out = append(out, "--root", r.Root)
	}
	return out
}

func (r *Runhcs) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, getCommandPath(), append(r.args(), args...)...)
	cmd.Env = os.Environ()
	return cmd
}

// runOrError will run the provided command.  If an error is
// encountered and neither Stdout or Stderr was set the error and the
// stderr of the command will be returned in the format of <error>:
// <stderr>.
func (r *Runhcs) runOrError(cmd *exec.Cmd) error {
	if cmd.Stdout != nil || cmd.Stderr != nil {
		ec, err := runc.Monitor.Start(cmd)
		if err != nil {
			return err
		}
		status, err := runc.Monitor.Wait(cmd, ec)
		if err == nil && status != 0 {
			err = fmt.Errorf("%s did not terminate successfully", cmd.Args[0])
		}
		return err
	}
	data, err := cmdOutput(cmd, true)
	if err != nil {
		return fmt.Errorf("%s: %s", err, data) //nolint:errorlint // legacy code
	}
	return nil
}

func cmdOutput(cmd *exec.Cmd, combined bool) ([]byte, error) {
	b := getBuf()
	defer putBuf(b)

	cmd.Stdout = b
	if combined {
		cmd.Stderr = b
	}
	ec, err := runc.Monitor.Start(cmd)
	if err != nil {
		return nil, err
	}

	status, err := runc.Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully", cmd.Args[0])
	}

	return b.Bytes(), err
}
