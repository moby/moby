package stdio

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/pkg/broadcaster"
	"github.com/docker/docker/pkg/pools"
	"github.com/moby/term"
	"github.com/sirupsen/logrus"
)

var _ Attacher = &local{}

// InitProcess defines the process ID for the primary process being managed by the Attacher
// This refers to the main container process.
// Other processes (execs) may use whatever unique value they like *except* this value.
const InitProcess = "init"

type local struct {
	mu      sync.RWMutex
	streams map[string]*streamAttacher
}

type streamAttacher struct {
	stdin            io.WriteCloser
	stdoutB, stderrB *broadcaster.Unbuffered
	stdout, stderr   io.ReadCloser
}

func (a *streamAttacher) Close() {
	if a.stdin != nil {
		a.stdin.Close()
	}
	if a.stdout != nil {
		a.stdoutB.Clean()
		a.stdout.Close()
	}
	if a.stderr != nil {
		a.stderrB.Clean()
		a.stderr.Close()
	}
}

// NewLocalAttacher creates an attacher from the passed in streams that attaches streams in process.
func NewLocalAttacher(stdin io.WriteCloser, stdout, stderr io.ReadCloser) Attacher {
	a := &streamAttacher{stdin: stdin}

	if stdout != nil {
		a.stdout = stdout
		a.stdoutB = &broadcaster.Unbuffered{}
		go func() {
			pools.Copy(a.stdoutB, stdout)
			stdout.Close()
		}()
	}

	if stderr != nil {
		a.stderr = stderr
		a.stderrB = &broadcaster.Unbuffered{}
		go func() {
			pools.Copy(a.stderrB, stderr)
			stderr.Close()
		}()
	}

	streams := map[string]*streamAttacher{
		InitProcess: a,
	}

	return &local{streams: streams}
}

var (
	errMissingStream = errors.New("missing required stream")
	errNotFound      = errors.New("stream not found")
)

func (m *local) Attach(ctx context.Context, process string, stdin, stdout, stderr *os.File) error {
	m.mu.RLock()
	attacher, ok := m.streams[process]
	m.mu.RUnlock()
	if !ok {
		return errNotFound
	}

	if attacher.stdin != nil && stdin != nil {
		go func() {
			_, err := pools.Copy(attacher.stdin, stdin)
			if err != nil {
				log.G(ctx).WithError(err).Error("error copying stdin stream")
			}
			stdin.Close()
		}()
	}

	if attacher.stdout != nil && stdout != nil {
		attacher.stdoutB.Add(stdout)
	}

	if attacher.stderr != nil && stderr != nil {
		attacher.stderrB.Add(stderr)
	}

	return nil
}

func (m *local) AttachMultiplexed(ctx context.Context, process string, f *os.File, framing StreamFraming, detachKeys []byte, includeStdin, includeStdout, includeStderr bool) error {
	m.mu.RLock()
	attacher, ok := m.streams[process]
	m.mu.RUnlock()
	if !ok {
		return errNotFound
	}

	var rwc io.ReadWriteCloser = f

	// Convert this to a net.Conn if possible
	conn, err := net.FileConn(f)
	if err == nil {
		f.Close()
		rwc = conn
	}

	stdin, stdout, stderr := GetFramedStreams(rwc, framing, includeStdin, includeStdout, includeStderr)
	if stdin != nil && attacher.stdin != nil {
		var r io.Reader = stdin
		if len(detachKeys) > 0 {
			r = term.NewEscapeProxy(r, detachKeys)
		}
		go func() {
			n, err := pools.Copy(attacher.stdin, r)
			logrus.WithError(err).WithField("copied", n).Debug("Finished copying stdin data")
			if err := stdin.Close(); err != nil {
				logrus.WithError(err).Debug("Error closing stdin")
			}
			logrus.Debug("stdin done")
		}()
	}
	if stdout != nil && attacher.stdout != nil {
		attacher.stdoutB.Add(stdout)
	}
	if stderr != nil && attacher.stderr != nil {
		attacher.stderrB.Add(stderr)
	}
	return nil
}

func (m *local) OpenStreams(ctx context.Context, process, stdinPath, stdoutPath, stderrPath string) (retErr error) {
	var (
		stdin            io.WriteCloser
		stdout, stderr   io.ReadCloser
		stdoutB, stderrB *broadcaster.Unbuffered
		err              error
	)
	defer func() {
		if retErr != nil {
			if stdin != nil {
				stdin.Close()
			}
			if stdout != nil {
				stdout.Close()
			}
			if stderr != nil {
				stderr.Close()
			}
		}
	}()

	if stdinPath != "" {
		stdin, err = openWriter(ctx, stdinPath)
		if err != nil {
			return err
		}
	}
	if stdoutPath != "" {
		stdout, err = openReader(ctx, stdoutPath)
		if err != nil {
			return err
		}
	}
	if stderrPath != "" {
		stdout, err = openReader(ctx, stderrPath)
		if err != nil {
			return err
		}
	}

	// Everything is open, so start up goroutines
	if stdout != nil {
		stdoutB = &broadcaster.Unbuffered{}
		go pools.Copy(stdoutB, stdout)
	}
	if stderr != nil {
		stderrB = &broadcaster.Unbuffered{}
		go pools.Copy(stderrB, stderr)
	}

	a := &streamAttacher{
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		stdoutB: stdoutB,
		stderrB: stderrB,
	}

	m.mu.Lock()
	m.streams[process] = a
	m.mu.Unlock()
	return nil
}

func (m *local) CloseStreams(ctx context.Context, process string) error {
	m.mu.Lock()
	a := m.streams[process]
	delete(m.streams, process)
	m.mu.Unlock()

	if a != nil {
		a.Close()
	}
	return nil
}

func (m *local) Close() error {
	m.mu.Lock()
	for key, s := range m.streams {
		s.Close()
		delete(m.streams, key)
	}
	m.mu.Unlock()

	return nil
}
