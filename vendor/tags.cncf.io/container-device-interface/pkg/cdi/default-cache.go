/*
   Copyright © 2024 The CDI Authors

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

package cdi

import (
	"sync"

	oci "github.com/opencontainers/runtime-spec/specs-go"
)

var (
	defaultCache   *Cache
	getDefaultOnce sync.Once
)

func getOrCreateDefaultCache(options ...Option) (*Cache, bool) {
	var created bool
	getDefaultOnce.Do(func() {
		defaultCache = newCache(options...)
		created = true
	})
	return defaultCache, created
}

// GetDefaultCache returns the default CDI cache instance.
func GetDefaultCache() *Cache {
	cache, _ := getOrCreateDefaultCache()
	return cache
}

// Configure applies options to the default CDI cache. Updates and refreshes
// the default cache if options are not empty.
func Configure(options ...Option) error {
	cache, created := getOrCreateDefaultCache(options...)
	if len(options) == 0 || created {
		return nil
	}
	return cache.Configure(options...)
}

// Refresh explicitly refreshes the default CDI cache instance.
func Refresh() error {
	return GetDefaultCache().Refresh()
}

// InjectDevices injects the given qualified devices to the given OCI Spec.
// using the default CDI cache instance to resolve devices.
func InjectDevices(ociSpec *oci.Spec, devices ...string) ([]string, error) {
	return GetDefaultCache().InjectDevices(ociSpec, devices...)
}

// GetErrors returns all errors encountered during the last refresh of
// the default CDI cache instance.
func GetErrors() map[string][]error {
	return GetDefaultCache().GetErrors()
}
