//go:build !windows

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
	"os/signal"
	"syscall"

	"github.com/containerd/containerd/sys/reaper"
	"github.com/containerd/fifo"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// setupSignals creates a new signal handler for all signals and sets the shim as a
// sub-reaper so that the container processes are reparented
func setupSignals(config Config) (chan os.Signal, error) {
	signals := make(chan os.Signal, 32)
	smp := []os.Signal{unix.SIGTERM, unix.SIGINT, unix.SIGPIPE}
	if !config.NoReaper {
		smp = append(smp, unix.SIGCHLD)
	}
	signal.Notify(signals, smp...)
	return signals, nil
}

func setupDumpStacks(dump chan<- os.Signal) {
	signal.Notify(dump, syscall.SIGUSR1)
}

func serveListener(path string) (net.Listener, error) {
	var (
		l   net.Listener
		err error
	)
	if path == "" {
		l, err = net.FileListener(os.NewFile(3, "socket"))
		path = "[inherited from parent]"
	} else {
		if len(path) > socketPathLimit {
			return nil, fmt.Errorf("%q: unix socket path too long (> %d)", path, socketPathLimit)
		}
		l, err = net.Listen("unix", path)
	}
	if err != nil {
		return nil, err
	}
	logrus.WithField("socket", path).Debug("serving api on socket")
	return l, nil
}

func reap(ctx context.Context, logger *logrus.Entry, signals chan os.Signal) error {
	logger.Debug("starting signal loop")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s := <-signals:
			// Exit signals are handled separately from this loop
			// They get registered with this channel so that we can ignore such signals for short-running actions (e.g. `delete`)
			switch s {
			case unix.SIGCHLD:
				if err := reaper.Reap(); err != nil {
					logger.WithError(err).Error("reap exit status")
				}
			case unix.SIGPIPE:
			}
		}
	}
}

func handleExitSignals(ctx context.Context, logger *logrus.Entry, cancel context.CancelFunc) {
	ch := make(chan os.Signal, 32)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case s := <-ch:
			logger.WithField("signal", s).Debugf("Caught exit signal")
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}
}

func openLog(ctx context.Context, _ string) (io.Writer, error) {
	return fifo.OpenFifoDup2(ctx, "log", unix.O_WRONLY, 0700, int(os.Stderr.Fd()))
}
