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
	"net"
	"os"
	"syscall"
	"time"

	winio "github.com/Microsoft/go-winio"
)

const shimBinaryFormat = "containerd-shim-%s-%s.exe"

func getSysProcAttr() *syscall.SysProcAttr {
	return nil
}

// AnonReconnectDialer connects to a named pipe that should already exist.
// It fails immediately if the pipe is not found, rather than retrying.
//
// Use this when reconnecting to a shim that is expected to be running
// (e.g. after a containerd restart). If the pipe doesn't exist, the shim
// is dead and there's no point waiting.
//
// This fail-fast behavior is critical on Windows: the Service Control Manager
// enforces a ~30s startup deadline on containerd. If reconnecting to many dead
// shims, a 5s retry per shim (as in AnonDialer) could exceed that budget and
// cause the SCM to kill containerd. See #3659.
//
// On Unix, this function simply calls AnonDialer since Unix domain sockets
// appear atomically and the distinction is unnecessary.
func AnonReconnectDialer(address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c, err := winio.DialPipeContext(ctx, address)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("npipe not found on reconnect: %w", os.ErrNotExist)
	} else if err == context.DeadlineExceeded {
		return nil, fmt.Errorf("timed out waiting for npipe %s: %w", address, err)
	} else if err != nil {
		return nil, err
	}
	return c, nil
}

// AnonDialer connects to a named pipe, retrying for up to 5 seconds if the
// pipe does not yet exist.
//
// Use this when connecting to a newly started shim. The shim's "start" helper
// returns the pipe address before the long-lived shim daemon has created the
// pipe, so a brief retry window is expected. 5 seconds is generous enough for
// any healthy shim to begin serving.
//
// Unlike Unix domain sockets (which appear atomically on Listen), Windows named
// pipes may take measurable time to appear — especially under load on CI
// runners. See #2519, #2692.
func AnonDialer(address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// If there is nobody serving the pipe we limit the timeout for this case to
	// 5 seconds because any shim that would serve this endpoint should serve it
	// within 5 seconds.
	serveTimer := time.NewTimer(5 * time.Second)
	defer serveTimer.Stop()
	for {
		c, err := winio.DialPipeContext(ctx, address)
		if err != nil {
			if os.IsNotExist(err) {
				select {
				case <-serveTimer.C:
					return nil, fmt.Errorf("pipe not found before timeout: %w", os.ErrNotExist)
				default:
					// Wait 10ms for the shim to serve and try again.
					time.Sleep(10 * time.Millisecond)
					continue
				}
			} else if err == context.DeadlineExceeded {
				return nil, fmt.Errorf("timed out waiting for npipe %s: %w", address, err)
			}
			return nil, err
		}
		return c, nil
	}
}

// RemoveSocket removes the socket at the specified address if
// it exists on the filesystem
func RemoveSocket(address string) error {
	return nil
}

func writeSocketDir(string) error {
	return nil
}

func cleanupSockets(context.Context) {
	if address, err := ReadAddress("address"); err == nil {
		_ = RemoveSocket(address)
	}
}
