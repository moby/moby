// Portions from https://github.com/containerd/nerdctl/pull/2723
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"github.com/vishvananda/netns"
)

// DetachedNetNS returns non-empty netns path if RootlessKit is running with --detach-netns mode.
// Otherwise returns "" without an error.
var DetachedNetNS = sync.OnceValues(detachedNetNS)

func detachedNetNS() (string, error) {
	stateDir := os.Getenv("ROOTLESSKIT_STATE_DIR")
	if stateDir == "" {
		return "", nil
	}
	p := filepath.Join(stateDir, "netns")
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return p, nil
}

// RunInNetNS runs f in the detached network namespace if one is
// configured, otherwise runs f directly. The function is executed on a
// dedicated OS thread that is discarded after use, because setns back to the
// host network namespace fails with EPERM in rootless mode (the host netns is
// owned by the initial user namespace where the daemon lacks CAP_SYS_ADMIN).
func RunInNetNS(nsPath string, f func() error) error {
	if nsPath == "" {
		return f()
	}

	ch := make(chan error, 1)
	go func() {
		runtime.LockOSThread()

		ns, err := netns.GetFromPath(nsPath)
		if err != nil {
			runtime.UnlockOSThread()
			ch <- fmt.Errorf("failed to open detached network namespace %s: %w", nsPath, err)
			return
		}
		defer ns.Close()

		origNS, err := netns.Get()
		if err != nil {
			runtime.UnlockOSThread()
			ch <- fmt.Errorf("failed to get current network namespace: %w", err)
			return
		}
		defer origNS.Close()

		if err := netns.Set(ns); err != nil {
			runtime.UnlockOSThread()
			ch <- fmt.Errorf("failed to enter detached network namespace: %w", err)
			return
		}

		ch <- f()

		if err := netns.Set(origNS); err != nil {
			// Cannot restore the thread's network namespace. Keep the
			// goroutine locked so the Go runtime terminates the thread
			// instead of returning a tainted thread to the pool.
			return
		}
		runtime.UnlockOSThread()
	}()
	return <-ch
}

// sandboxNSThreads tracks OS threads that are currently executing in a
// container sandbox network namespace (via InvokeFunc). This is used to
// prevent iptables/nftables wrappers from adding nsenter to the detached
// netns when they are already running in the correct (container) namespace.
var sandboxNSThreads sync.Map // key: int (tid)

// MarkInSandboxNS marks the current OS thread as being inside a container
// sandbox network namespace. The caller must have locked the OS thread with
// runtime.LockOSThread before calling this function.
func MarkInSandboxNS() {
	sandboxNSThreads.Store(syscall.Gettid(), struct{}{})
}

// UnmarkInSandboxNS removes the sandbox namespace mark from the current OS
// thread. Must be called on the same locked thread that called MarkInSandboxNS.
func UnmarkInSandboxNS() {
	sandboxNSThreads.Delete(syscall.Gettid())
}

// InSandboxNS reports whether the current OS thread has been marked as
// executing inside a container sandbox network namespace. When true,
// iptables/nftables commands should NOT be wrapped with nsenter to the
// detached netns, because the thread is already in the target (container)
// namespace.
func InSandboxNS() bool {
	_, ok := sandboxNSThreads.Load(syscall.Gettid())
	return ok
}
