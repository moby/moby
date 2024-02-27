// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"sync"
)

// cache is a locking storage used to quickly return already computed values.
//
// The zero value of a cache is empty and ready to use.
//
// A cache must not be copied after first use.
//
// All methods of a cache are safe to call concurrently.
type cache[K comparable, V any] struct {
	sync.Mutex
	data map[K]V
}

// Lookup returns the value stored in the cache with the associated key if it
// exists. Otherwise, f is called and its returned value is set in the cache
// for key and returned.
//
// Lookup is safe to call concurrently. It will hold the cache lock, so f
// should not block excessively.
func (c *cache[K, V]) Lookup(key K, f func() V) V {
	c.Lock()
	defer c.Unlock()

	if c.data == nil {
		val := f()
		c.data = map[K]V{key: val}
		return val
	}
	if v, ok := c.data[key]; ok {
		return v
	}
	val := f()
	c.data[key] = val
	return val
}
