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

import (
	"fmt"
)

type RDMA struct {
	Limit []RDMAEntry
}

type RDMAEntry struct {
	Device     string
	HcaHandles uint32
	HcaObjects uint32
}

func (r RDMAEntry) String() string {
	return fmt.Sprintf("%s hca_handle=%d hca_object=%d", r.Device, r.HcaHandles, r.HcaObjects)
}

func (r *RDMA) Values() (o []Value) {
	for _, e := range r.Limit {
		o = append(o, Value{
			filename: "rdma.max",
			value:    e.String(),
		})
	}

	return o
}
