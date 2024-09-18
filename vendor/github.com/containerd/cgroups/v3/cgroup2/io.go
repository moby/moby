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

import "fmt"

type IOType string

const (
	ReadBPS   IOType = "rbps"
	WriteBPS  IOType = "wbps"
	ReadIOPS  IOType = "riops"
	WriteIOPS IOType = "wiops"
)

type BFQ struct {
	Weight uint16
}

type Entry struct {
	Type  IOType
	Major int64
	Minor int64
	Rate  uint64
}

func (e Entry) String() string {
	return fmt.Sprintf("%d:%d %s=%d", e.Major, e.Minor, e.Type, e.Rate)
}

type IO struct {
	BFQ BFQ
	Max []Entry
}

func (i *IO) Values() (o []Value) {
	if i.BFQ.Weight != 0 {
		o = append(o, Value{
			filename: "io.bfq.weight",
			value:    i.BFQ.Weight,
		})
	}
	for _, e := range i.Max {
		o = append(o, Value{
			filename: "io.max",
			value:    e.String(),
		})
	}
	return o
}
