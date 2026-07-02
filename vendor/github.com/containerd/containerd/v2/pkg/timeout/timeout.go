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

package timeout

import (
	"context"
	"sync"
	"time"
)

var (
	mu       sync.RWMutex
	timeouts = make(map[string]time.Duration)

	// DefaultTimeout of the timeout package
	DefaultTimeout = 1 * time.Second
)

// Set the timeout for the key
func Set(key string, t time.Duration) {
	mu.Lock()
	timeouts[key] = t
	mu.Unlock()
}

// Get returns the timeout for the provided key
func Get(key string) time.Duration {
	mu.RLock()
	t, ok := timeouts[key]
	mu.RUnlock()
	if !ok {
		t = DefaultTimeout
	}
	return t
}

// WithContext returns a context with the specified timeout for the provided key
func WithContext(ctx context.Context, key string) (context.Context, func()) {
	t := Get(key)
	return context.WithTimeout(ctx, t)
}

// All returns all keys and their timeouts
func All() map[string]time.Duration {
	out := make(map[string]time.Duration)
	mu.RLock()
	defer mu.RUnlock()
	for k, v := range timeouts {
		out[k] = v
	}
	return out
}
