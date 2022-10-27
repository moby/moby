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
	"io"
	"net"
	"os"

	"github.com/containerd/ttrpc"
	"github.com/sirupsen/logrus"
)

func setupSignals(config Config) (chan os.Signal, error) {
	return nil, errors.New("not supported")
}

func newServer(opts ...ttrpc.ServerOpt) (*ttrpc.Server, error) {
	return nil, errors.New("not supported")
}

func subreaper() error {
	return errors.New("not supported")
}

func setupDumpStacks(dump chan<- os.Signal) {
}

func serveListener(path string) (net.Listener, error) {
	return nil, errors.New("not supported")
}

func reap(ctx context.Context, logger *logrus.Entry, signals chan os.Signal) error {
	return errors.New("not supported")
}

func handleExitSignals(ctx context.Context, logger *logrus.Entry, cancel context.CancelFunc) {
}

func openLog(ctx context.Context, _ string) (io.Writer, error) {
	return nil, errors.New("not supported")
}
