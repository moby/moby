// Copyright 2025, Google Inc.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package gax

import (
	"os"
	"strings"
	"sync"
)

var (
	// featureEnabledOnce caches results for IsFeatureEnabled.
	featureEnabledOnce  sync.Once
	featureEnabledStore map[string]bool
)

// IsFeatureEnabled checks if an experimental feature is enabled via
// environment variable. The environment variable must be prefixed with
// "GOOGLE_SDK_GO_EXPERIMENTAL_". The feature name passed to this
// function must be the suffix (e.g., "FOO" for "GOOGLE_SDK_GO_EXPERIMENTAL_FOO").
// To enable the feature, the environment variable's value must be "true",
// case-insensitive. The result for each name is cached on the first call.
func IsFeatureEnabled(name string) bool {
	featureEnabledOnce.Do(func() {
		featureEnabledStore = make(map[string]bool)
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "GOOGLE_SDK_GO_EXPERIMENTAL_") {
				// Parse "KEY=VALUE"
				kv := strings.SplitN(env, "=", 2)
				if len(kv) == 2 && strings.ToLower(kv[1]) == "true" {
					key := strings.TrimPrefix(kv[0], "GOOGLE_SDK_GO_EXPERIMENTAL_")
					featureEnabledStore[key] = true
				}
			}
		}
	})
	return featureEnabledStore[name]
}

// TestOnlyResetIsFeatureEnabled is for testing purposes only. It resets the cached
// feature flags, allowing environment variables to be re-read on the next call to IsFeatureEnabled.
// This function is not thread-safe; if another goroutine reads a feature after this
// function is called but before the `featureEnabledOnce` is re-initialized by IsFeatureEnabled,
// it may see an inconsistent state.
func TestOnlyResetIsFeatureEnabled() {
	featureEnabledOnce = sync.Once{}
	featureEnabledStore = nil
}
