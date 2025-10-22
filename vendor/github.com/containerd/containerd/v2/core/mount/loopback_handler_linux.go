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

package mount

import (
	"context"
	"os"
	"time"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
)

func LoopbackHandler() Handler {
	return loopbackHandler{}
}

type loopbackHandler struct {
}

func (loopbackHandler) Mount(ctx context.Context, m Mount, mp string, _ []ActiveMount) (ActiveMount, error) {
	if m.Type != "loop" {
		return ActiveMount{}, errdefs.ErrNotImplemented
	}
	params := LoopParams{
		Autoclear: true,
	}
	// TODO: Handle readonly
	// TODO: Handle direct io

	t := time.Now()
	loop, err := setupLoop(m.Source, params)
	if err != nil {
		return ActiveMount{}, err
	}
	defer loop.Close()

	if err := os.Symlink(loop.Name(), mp); err != nil {
		return ActiveMount{}, err
	}

	if err := setLoopAutoclear(loop, false); err != nil {
		return ActiveMount{}, err
	}

	return ActiveMount{
		Mount:      m,
		MountedAt:  &t,
		MountPoint: mp,
	}, nil
}

func (loopbackHandler) Unmount(ctx context.Context, path string) error {
	loopdev, err := os.Readlink(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	loop, err := os.Open(loopdev)
	if err != nil {
		return err
	}
	defer loop.Close()

	if err := setLoopAutoclear(loop, true); err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		// if removal of the symlink has failed, its possible for the loop device to get cleaned
		// up and re-used. Leave the loop device around to prevent re-use and let a retry of
		// Unmount clear it.`
		if err := setLoopAutoclear(loop, false); err != nil {
			// Very unlikely but log to track in case there is a problem with
			// the loop being cleared and re-used.
			log.G(ctx).WithError(err).Errorf("Failed to unset auto clear flag on symlink removal failure, loopback %q may be cleaned up while still being tracked", loopdev)
		}

		return err
	}

	return nil
}
