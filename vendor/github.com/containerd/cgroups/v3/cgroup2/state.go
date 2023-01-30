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

package cgroup2

import (
	"os"
	"path/filepath"
	"strings"
)

// State is a type that represents the state of the current cgroup
type State string

const (
	Unknown State = ""
	Thawed  State = "thawed"
	Frozen  State = "frozen"
	Deleted State = "deleted"

	cgroupFreeze = "cgroup.freeze"
)

func (s State) Values() []Value {
	v := Value{
		filename: cgroupFreeze,
	}
	switch s {
	case Frozen:
		v.value = "1"
	case Thawed:
		v.value = "0"
	}
	return []Value{
		v,
	}
}

func fetchState(path string) (State, error) {
	current, err := os.ReadFile(filepath.Join(path, cgroupFreeze))
	if err != nil {
		return Unknown, err
	}
	switch strings.TrimSpace(string(current)) {
	case "1":
		return Frozen, nil
	case "0":
		return Thawed, nil
	default:
		return Unknown, nil
	}
}
