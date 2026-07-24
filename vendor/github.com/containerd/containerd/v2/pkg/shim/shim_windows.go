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
	"fmt"
	"io"
	"net"
	"os"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/ttrpc"
)

func setupSignals(config Config) (chan os.Signal, error) {
	return nil, errdefs.ErrNotImplemented
}

func newServer(opts ...ttrpc.ServerOpt) (*ttrpc.Server, error) {
	return nil, errdefs.ErrNotImplemented
}

func subreaper() error {
	return errdefs.ErrNotImplemented
}

func setupDumpStacks(dump chan<- os.Signal) {
}

func serveListener(path string, fd uintptr) (net.Listener, error) {
	return nil, errdefs.ErrNotImplemented
}

func reap(ctx context.Context, logger *log.Entry, signals chan os.Signal) error {
	return errdefs.ErrNotImplemented
}

func handleExitSignals(ctx context.Context, logger *log.Entry, cancel context.CancelFunc) {
}

func openLog(ctx context.Context, _ string) (io.Writer, error) {
	return nil, errdefs.ErrNotImplemented
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
		// Retry on both "pipe not found" and "pipe busy / deadline exceeded"
		// — the pipe may still be starting up or temporarily at capacity.
		if !os.IsNotExist(err) && err != context.DeadlineExceeded {
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
