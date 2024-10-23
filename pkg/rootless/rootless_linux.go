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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/sirupsen/logrus"
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
func WithDetachedNetNSIfAny(fn func() error) error {
	netns, err := DetachedNetNS()
	if err != nil {
		return err
	}
	if netns == "" {
		return fn()
	}
	return ns.WithNetNSPath(netns, func(_ ns.NetNS) error {
		// ns.WithNetNSPath should have handled the thread locks,
		// but apparently that is not enough.
		// https://github.com/moby/moby/pull/47103#issuecomment-2428610609
		logrus.Debugf("ENTER detached netns")
		runtime.LockOSThread()
		defer func() {
			runtime.UnlockOSThread()
			logrus.Debugf("LEAVE detached netns")
		}()
		return fn()
	})
}
