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

package v2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/pkg/namespaces"
)

type deferredPipeConnection struct {
	ctx context.Context

	wg   sync.WaitGroup
	once sync.Once

	c      net.Conn
	conerr error
}

func (dpc *deferredPipeConnection) Read(p []byte) (n int, err error) {
	if dpc.c == nil {
		dpc.wg.Wait()
		if dpc.c == nil {
			return 0, dpc.conerr
		}
	}
	return dpc.c.Read(p)
}
func (dpc *deferredPipeConnection) Close() error {
	var err error
	dpc.once.Do(func() {
		dpc.wg.Wait()
		if dpc.c != nil {
			err = dpc.c.Close()
		} else if dpc.conerr != nil {
			err = dpc.conerr
		}
	})
	return err
}

// openShimLog on Windows acts as the client of the log pipe. In this way the
// containerd daemon can reconnect to the shim log stream if it is restarted.
func openShimLog(ctx context.Context, bundle *Bundle, dialer func(string, time.Duration) (net.Conn, error)) (io.ReadCloser, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	dpc := &deferredPipeConnection{
		ctx: ctx,
	}
	dpc.wg.Add(1)
	go func() {
		c, conerr := dialer(
			fmt.Sprintf("\\\\.\\pipe\\containerd-shim-%s-%s-log", ns, bundle.ID),
			time.Second*10,
		)
		if conerr != nil {
			dpc.conerr = fmt.Errorf("failed to connect to shim log: %w", conerr)
		}
		dpc.c = c
		dpc.wg.Done()
	}()
	return dpc, nil
}

func checkCopyShimLogError(ctx context.Context, err error) error {
	// When using a multi-container shim the 2nd to Nth container in the
	// shim will not have a separate log pipe. Ignore the failure log
	// message here when the shim connect times out.
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
