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

package local

import (
	"sync"

	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
)

// Handles locking references

var (
	// locks lets us lock in process
	locks   = map[string]struct{}{}
	locksMu sync.Mutex
)

func tryLock(ref string) error {
	locksMu.Lock()
	defer locksMu.Unlock()

	if _, ok := locks[ref]; ok {
		return errors.Wrapf(errdefs.ErrUnavailable, "ref %s locked", ref)
	}

	locks[ref] = struct{}{}
	return nil
}

func unlock(ref string) {
	locksMu.Lock()
	defer locksMu.Unlock()

	if _, ok := locks[ref]; ok {
		delete(locks, ref)
	}
}
