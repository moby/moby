// Copyright 2020 Intel Corporation. All Rights Reserved.
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

package utils

import (
	"sort"
)

// SortUint64s sorts a slice of uint64 in increasing order.
func SortUint64s(a []uint64) {
	sort.Sort(Uint64Slice(a))
}

// Uint64Slice implmenents sort.Interface for a slice of uint64.
type Uint64Slice []uint64

// Len returns the length of an UintSlice
func (s Uint64Slice) Len() int { return len(s) }

// Less returns true if element at 'i' is less than the element at 'j'
func (s Uint64Slice) Less(i, j int) bool { return s[i] < s[j] }

// Swap swaps the values of two elements
func (s Uint64Slice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
