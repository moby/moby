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

// AnonReconnectDialer returns a dialer for an existing npipe on containerd reconnection
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

// AnonDialer returns a dialer for a npipe
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

func cleanupSockets(context.Context) {
	if address, err := ReadAddress("address"); err == nil {
		_ = RemoveSocket(address)
	}
}
