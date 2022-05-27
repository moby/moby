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

package v2

type Memory struct {
	Swap *int64
	Min  *int64
	Max  *int64
	Low  *int64
	High *int64
}

func (r *Memory) Values() (o []Value) {
	if r.Swap != nil {
		o = append(o, Value{
			filename: "memory.swap.max",
			value:    *r.Swap,
		})
	}
	if r.Min != nil {
		o = append(o, Value{
			filename: "memory.min",
			value:    *r.Min,
		})
	}
	if r.Max != nil {
		o = append(o, Value{
			filename: "memory.max",
			value:    *r.Max,
		})
	}
	if r.Low != nil {
		o = append(o, Value{
			filename: "memory.low",
			value:    *r.Low,
		})
	}
	if r.High != nil {
		o = append(o, Value{
			filename: "memory.high",
			value:    *r.High,
		})
	}
	return o
}
