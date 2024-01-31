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

// Package randutil provides utilities for [cyrpto/rand].
package randutil

import (
	"crypto/rand"
	"math"
	"math/big"
)

// Int63n is similar to [math/rand.Int63n] but uses [crypto/rand.Reader] under the hood.
func Int63n(n int64) int64 {
	b, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		panic(err)
	}
	return b.Int64()
}

// Int63 is similar to [math/rand.Int63] but uses [crypto/rand.Reader] under the hood.
func Int63() int64 {
	return Int63n(math.MaxInt64)
}

// Intn is similar to [math/rand.Intn] but uses [crypto/rand.Reader] under the hood.
func Intn(n int) int {
	return int(Int63n(int64(n)))
}

// Int is similar to [math/rand.Int] but uses [crypto/rand.Reader] under the hood.
func Int() int {
	return int(Int63())
}
