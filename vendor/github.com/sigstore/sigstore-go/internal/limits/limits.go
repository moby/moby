// Copyright 2026 The Sigstore Authors.
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

// Package limits centralizes the per-bundle upper bounds shared between
// the bundle parse path and the verify path.
package limits

// MaxAllowedTlogEntries is the upper bound on the number of transparency
// log entries a bundle may carry.
const MaxAllowedTlogEntries = 32

// MaxAllowedTimestamps is the upper bound on the number of signed
// timestamps a bundle may carry.
const MaxAllowedTimestamps = 32
