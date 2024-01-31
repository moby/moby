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

// Package kmutex provides synchronization primitives to lock/unlock resource by unique key.
package kmutex

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/semaphore"
)

// KeyedLocker is the interface for acquiring locks based on string.
type KeyedLocker interface {
	Lock(ctx context.Context, key string) error
	Unlock(key string)
}

func New() KeyedLocker {
	return newKeyMutex()
}

func newKeyMutex() *keyMutex {
	return &keyMutex{
		locks: make(map[string]*klock),
	}
}

type keyMutex struct {
	mu sync.Mutex

	locks map[string]*klock
}

type klock struct {
	*semaphore.Weighted
	ref int
}

func (km *keyMutex) Lock(ctx context.Context, key string) error {
	km.mu.Lock()

	l, ok := km.locks[key]
	if !ok {
		km.locks[key] = &klock{
			Weighted: semaphore.NewWeighted(1),
		}
		l = km.locks[key]
	}
	l.ref++
	km.mu.Unlock()

	if err := l.Acquire(ctx, 1); err != nil {
		km.mu.Lock()
		defer km.mu.Unlock()

		l.ref--

		if l.ref < 0 {
			panic(fmt.Errorf("kmutex: release of unlocked key %v", key))
		}

		if l.ref == 0 {
			delete(km.locks, key)
		}
		return err
	}
	return nil
}

func (km *keyMutex) Unlock(key string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	l, ok := km.locks[key]
	if !ok {
		panic(fmt.Errorf("kmutex: unlock of unlocked key %v", key))
	}
	l.Release(1)

	l.ref--

	if l.ref < 0 {
		panic(fmt.Errorf("kmutex: released of unlocked key %v", key))
	}

	if l.ref == 0 {
		delete(km.locks, key)
	}
}
