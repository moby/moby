//
// Copyright 2025 The Sigstore Authors.
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

package safeint

import (
	"fmt"
	"math"
)

// SafeInt64 holds equivalent int64 and uint64 integers.
type SafeInt64 struct {
	u uint64
	i int64
}

// NewSafeInt64 returns a safeInt64 struct as long as the number is either an
// int64 or uint64 and the value can safely be converted in either direction
// without overflowing, i.e. is not greater than MaxInt64 and not negative.
//
// This has implications for its usage, e.g. when used for the tree size, a new
// tree must be created to replace the old tree before its size reaches
// math.MaxInt64.
//
// This is needed for compatibility with TransparencyLogEntry
// (https://github.com/sigstore/protobuf-specs/blob/e871d3e6fd06fa73a1524ef0efaf1452d3304cf6/protos/sigstore_rekor.proto#L86-L138).
func NewSafeInt64(number any) (*SafeInt64, error) {
	var result SafeInt64
	switch n := number.(type) {
	case uint64:
		if n > math.MaxInt64 {
			return nil, fmt.Errorf("exceeded max int64: %d", n)
		}
		result.u = n
		result.i = int64(n) //nolint:gosec
	case int64:
		if n < 0 {
			return nil, fmt.Errorf("negative integer: %d", n)
		}
		result.u = uint64(n) //nolint:gosec
		result.i = n
	default:
		return nil, fmt.Errorf("only uint64 and int64 are supported")
	}
	return &result, nil
}

// U returns the uint64 value of the integer.
func (s *SafeInt64) U() uint64 {
	return s.u
}

// I returns the int64 value of the integer.
func (s *SafeInt64) I() int64 {
	return s.i
}
