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
	"fmt"
	"sync"
	"time"

	"github.com/containerd/containerd/errdefs"
)

// Handles locking references

type lock struct {
	since time.Time
}

var (
	// locks lets us lock in process
	locks   = make(map[string]*lock)
	locksMu sync.Mutex
)

func tryLock(ref string) error {
	locksMu.Lock()
	defer locksMu.Unlock()

	if v, ok := locks[ref]; ok {
		// Returning the duration may help developers distinguish dead locks (long duration) from
		// lock contentions (short duration).
		now := time.Now()
		return fmt.Errorf(
			"ref %s locked for %s (since %s): %w", ref, now.Sub(v.since), v.since,
			errdefs.ErrUnavailable,
		)
	}

	locks[ref] = &lock{time.Now()}
	return nil
}

func unlock(ref string) {
	locksMu.Lock()
	defer locksMu.Unlock()

	delete(locks, ref)
}
