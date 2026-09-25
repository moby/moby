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

import "strconv"

type Pids struct {
	Max int64
}

func (r *Pids) Values() (o []Value) {
	if r.Max != 0 {
		limit := "max"
		if r.Max > 0 {
			limit = strconv.FormatInt(r.Max, 10)
		}
		o = append(o, Value{
			filename: "pids.max",
			value:    limit,
		})
	}
	return o
}
