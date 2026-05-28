// Copyright 2021 Google LLC. All Rights Reserved.
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

package log

import (
	"crypto/sha256"
	"fmt"
)

// ID returns the identifier to use for a log given the Origin. This is the ID
// used to find checkpoints for this log at distributors, and that will be used
// to feed checkpoints to witnesses.
func ID(origin string) string {
	s := sha256.New()
	s.Write([]byte("o:"))
	s.Write([]byte(origin))
	return fmt.Sprintf("%x", s.Sum(nil))
}
