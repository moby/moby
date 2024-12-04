// From https://github.com/containerd/nerdctl/pull/2723
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

package rootless

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/containerd/log"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netns"
)

var (
	detachedNetNS     string
	detachedNetNSErr  error
	detachedNetNSOnce sync.Once
)

// DetachedNetNS returns non-empty netns path if RootlessKit is running with --detach-netns mode.
// Otherwise returns "" without an error.
func DetachedNetNS() (string, error) {
	detachedNetNSOnce.Do(func() {
		stateDir := os.Getenv("ROOTLESSKIT_STATE_DIR")
		if stateDir == "" {
			return
		}
		p := filepath.Join(stateDir, "netns")
		if _, err := os.Stat(p); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return
			}
			detachedNetNSErr = err
			return
		}
		detachedNetNS = p
	})
	return detachedNetNS, detachedNetNSErr
}

// WithDetachedNetNSIfAny executes fn in [DetachedNetNS] if RootlessKit is running with --detach-netns mode.
// Otherwise it just executes fn in the current netns.
func WithDetachedNetNSIfAny(f func() error) error {
	netns, err := DetachedNetNS()
	if err != nil {
		return err
	}
	if netns == "" {
		return f()
	}

	// <DEBUG>
	debug.PrintStack()
	// </DEBUG>

	var fErr error
	if err := WithNetNSPath(netns, func() {
		// <DEBUG>
		logrus.Debugf("ENTER detached netns")
		defer func() {
			logrus.Debugf("LEAVE detached netns")
		}()
		// </DEBUG>

		fErr = f()
	}); err != nil {
		return err
	}
	return fErr
}

func init() {
	// https://github.com/moby/moby/blob/v27.3.1/libnetwork/osl/namespace_linux.go#L37
	// TODO: deduplicate the code
	runtime.LockOSThread()
}

// WithNetNSPath derived from libnetwork/osl.(*Namespace).InvokeFunc
// https://github.com/moby/moby/blob/v27.3.1/libnetwork/osl/namespace_linux.go#L401
// TODO: deduplicate the code
func WithNetNSPath(path string, f func()) error {
	newNS, err := netns.GetFromPath(path)
	if err != nil {
		return fmt.Errorf("failed get network namespace %q: %w", path, err)
	}
	defer newNS.Close()

	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		// InvokeFunc() could have been called from a goroutine with
		// tampered thread state, e.g. from another InvokeFunc()
		// callback. The outer goroutine's thread state cannot be
		// trusted.
		origNS, err := netns.Get()
		if err != nil {
			runtime.UnlockOSThread()
			done <- fmt.Errorf("failed to get original network namespace: %w", err)
			return
		}
		defer origNS.Close()

		if err := netns.Set(newNS); err != nil {
			runtime.UnlockOSThread()
			done <- err
			return
		}
		defer func() {
			close(done)
			if err := netns.Set(origNS); err != nil {
				log.G(context.TODO()).WithError(err).Warn("failed to restore thread's network namespace")
				// Recover from the error by leaving this goroutine locked to
				// the thread. The runtime will terminate the thread and replace
				// it with a clean one when this goroutine returns.
			} else {
				runtime.UnlockOSThread()
			}
		}()
		f()
	}()
	return <-done
}
