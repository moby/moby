/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package shim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/log"
	"github.com/containerd/ttrpc"
	"golang.org/x/sys/windows"
)

// setupSignals creates the shim's signal channel for Windows and registers
// interrupt/terminate on it. Short-lived actions (e.g. "delete") run only
// reap(), which drains these so a stray Ctrl+C / termination cannot kill the
// process mid-action via the default OS behavior; serve() additionally handles
// graceful shutdown through handleExitSignals. Windows has no SIGCHLD (the OS
// reaps children), so no reaping signal is registered.
func setupSignals(_ Config) (chan os.Signal, error) {
	signals := make(chan os.Signal, 32)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	return signals, nil
}

// newServer creates a new ttrpc server for Windows.
// Unlike Unix, Windows doesn't have user-based socket authentication,
// so we create a basic ttrpc server without the handshaker.
func newServer(opts ...ttrpc.ServerOpt) (*ttrpc.Server, error) {
	return ttrpc.NewServer(opts...)
}

// subreaper is not applicable on Windows as the OS automatically
// handles orphaned processes differently than Unix systems.
func subreaper() error {
	// This is a no-op on Windows - the OS handles orphaned processes
	return nil
}

// setupDumpStacks is currently not implemented for Windows.
// Windows doesn't have SIGUSR1, so stack dumping would need to use
// a different mechanism (e.g., a named event or debug console).
func setupDumpStacks(_ chan<- os.Signal) {
	// No-op on Windows - SIGUSR1 doesn't exist
	// Future: could implement using Windows events or console signals
}

// serveListener creates a named pipe listener for Windows at the given path.
// Windows requires an explicit named-pipe path; unlike Unix there is no
// inherited-descriptor fallback, so an empty path is an error.
func serveListener(path string, _ uintptr) (net.Listener, error) {
	if path == "" {
		return nil, fmt.Errorf("named pipe path is required on Windows")
	}

	// Require the canonical Windows named pipe prefix: \\.\pipe\<name>.
	if !strings.HasPrefix(path, `\\.\pipe\`) {
		return nil, fmt.Errorf("address %q is not a named pipe path (must start with %q)", path, `\\.\pipe\`)
	}

	l, err := winio.ListenPipe(path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create named pipe listener at %s: %w", path, err)
	}

	log.L.WithField("pipe", path).Debug("serving api on named pipe")
	return l, nil
}

// reap handles signals on Windows. Unlike Unix, Windows doesn't send SIGCHLD
// when child processes exit, so we only need to handle shutdown signals.
func reap(ctx context.Context, logger *log.Entry, signals chan os.Signal) error {
	logger.Debug("starting signal loop")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s := <-signals:
			logger.WithField("signal", s).Debug("received signal in reap loop")
			// On Windows, we just log the signal
			// Exit signals are handled in handleExitSignals
		}
	}
}

// handleExitSignals listens for shutdown signals (SIGINT, SIGTERM) and
// triggers the provided cancel function for graceful shutdown.
func handleExitSignals(ctx context.Context, logger *log.Entry, cancel context.CancelFunc) {
	ch := make(chan os.Signal, 32)
	// On Windows, os.Kill cannot be caught. We handle os.Interrupt (Ctrl+C) and SIGTERM.
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case s := <-ch:
			logger.WithField("signal", s).Debug("caught exit signal")
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}
}

// openLog creates a named pipe for shim logging on Windows.
// The containerd daemon connects to this pipe as a client to read log output.
// The pipe format is: \\.\pipe\containerd-shim-{namespace}-{id}-log
func openLog(ctx context.Context, id string) (io.Writer, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	pipePath := fmt.Sprintf("\\\\.\\pipe\\containerd-shim-%s-%s-log", ns, id)
	l, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create shim log pipe: %w", err)
	}

	rlw := &reconnectingLogWriter{
		l: l,
	}

	// Accept connections from containerd in the background.
	// Supports reconnection if containerd restarts.
	go rlw.acceptConnections()

	return rlw, nil
}

// awaitPipeReady polls a named pipe address until it is connectable,
// retrying for up to 5 seconds with 10ms intervals.
//
// The shim "start" helper returns the pipe address before the long-lived
// daemon has called winio.ListenPipe(). Unlike Unix domain sockets (which
// appear atomically on Listen), Windows named pipes may take measurable
// time to appear — especially under load. See #3659, microsoft/hcsshim.
func awaitPipeReady(address string) error {
	if address == "" {
		return nil
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	var lastErr error
	for {
		// Use a 1s per-attempt timeout to avoid blocking indefinitely if
		// the pipe exists but all instances are busy.
		dialTimeout := time.Second
		conn, err := winio.DialPipe(address, &dialTimeout)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		// Retry on pipe-not-found, i/o timeout, and pipe-busy.
		// winio.DialPipe returns winio.ErrTimeout when the per-attempt timeout fires.
		// ERROR_PIPE_BUSY is normally absorbed by go-winio's tryDialPipe loop
		// and surfaces as winio.ErrTimeout once the deadline fires; guard it
		// explicitly in case a future go-winio version surfaces it unwrapped.
		retryable := os.IsNotExist(err) ||
			errors.Is(err, winio.ErrTimeout) ||
			errors.Is(err, windows.ERROR_PIPE_BUSY)
		if !retryable {
			return err
		}
		select {
		case <-timer.C:
			return fmt.Errorf("pipe %s not ready after 5s: %w", address, lastErr)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// reconnectingLogWriter adapts containerd's log channel to the Windows
// named-pipe model.
//
// On Unix the shim log is a FIFO: a passive kernel object the shim writes to
// while containerd reads the far end, and containerd can detach and reattach
// (for example across a restart) with no involvement from the shim. Windows
// named pipes have no such passive form — they are connection-oriented, so the
// shim is a server that must Accept a reader, serves one reader at a time, and
// must Accept a fresh connection each time that reader reconnects.
//
// This type provides exactly what the pipe model forces the shim to handle
// itself, and which the FIFO gave for free:
//   - a background Accept loop so a reader can connect at any time,
//   - swapping to the newest connection (closing the old) on reconnect,
//   - dropping writes while no reader is attached so logging never blocks the shim.
//
// Logs written while no reader is connected — including the brief window during
// a reconnect — are intentionally discarded rather than buffered.
type reconnectingLogWriter struct {
	l    net.Listener // named pipe listener that accepts reader connections
	mu   sync.Mutex   // guards conn
	conn net.Conn     // current reader connection, or nil when none is attached
}

// acceptConnections listens for log connections in the background.
func (rlw *reconnectingLogWriter) acceptConnections() {
	for {
		newConn, err := rlw.l.Accept()
		if err != nil {
			// Listener was closed, stop accepting
			return
		}

		rlw.mu.Lock()
		// Close the old connection if one exists
		if rlw.conn != nil {
			rlw.conn.Close()
		}
		rlw.conn = newConn
		rlw.mu.Unlock()
	}
}

// Write implements io.Writer. It writes to the current connection if one exists.
// If no connection is established yet, writes are silently dropped to avoid
// blocking the shim.
func (rlw *reconnectingLogWriter) Write(p []byte) (n int, err error) {
	rlw.mu.Lock()
	conn := rlw.conn
	rlw.mu.Unlock()

	if conn == nil {
		// No connection yet, drop the log.
		return len(p), nil
	}

	n, err = conn.Write(p)
	if err != nil || n < len(p) {
		// A write error or short write means the reader is gone or wedged.
		// Drop the connection so the next write starts fresh, and report full
		// success so logging never backpressures the shim.
		rlw.mu.Lock()
		if rlw.conn == conn {
			rlw.conn.Close()
			rlw.conn = nil
		}
		rlw.mu.Unlock()
		return len(p), nil
	}
	return len(p), nil
}

// Close implements io.Closer. It closes both the listener and any active connection.
func (rlw *reconnectingLogWriter) Close() error {
	rlw.mu.Lock()
	defer rlw.mu.Unlock()

	var err error
	if rlw.l != nil {
		err = rlw.l.Close()
	}
	if rlw.conn != nil {
		if cerr := rlw.conn.Close(); cerr != nil && err == nil {
			err = cerr
		}
		rlw.conn = nil
	}
	return err
}
