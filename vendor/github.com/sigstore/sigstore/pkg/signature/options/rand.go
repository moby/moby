//
// Copyright 2021 The Sigstore Authors.
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

package options

import (
	crand "crypto/rand"
	"io"
)

// RequestRand implements the functional option pattern for using a specific source of entropy
type RequestRand struct {
	NoOpOptionImpl
	rand io.Reader
}

// ApplyRand sets the specified source of entropy as the functional option
func (r RequestRand) ApplyRand(rand *io.Reader) {
	*rand = r.rand
}

// WithRand specifies that the given source of entropy should be used in signing operations
func WithRand(rand io.Reader) RequestRand {
	r := rand
	if r == nil {
		r = crand.Reader
	}
	return RequestRand{rand: r}
}
