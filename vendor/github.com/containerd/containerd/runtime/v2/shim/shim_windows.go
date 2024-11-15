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
	"io"
	"net"
	"os"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/ttrpc"
	"github.com/sirupsen/logrus"
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

func serveListener(path string) (net.Listener, error) {
	return nil, errdefs.ErrNotImplemented
}

func reap(ctx context.Context, logger *logrus.Entry, signals chan os.Signal) error {
	return errdefs.ErrNotImplemented
}

func handleExitSignals(ctx context.Context, logger *logrus.Entry, cancel context.CancelFunc) {
}

func openLog(ctx context.Context, _ string) (io.Writer, error) {
	return nil, errdefs.ErrNotImplemented
}
