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

package protobuf

import (
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
)

var Compare = cmp.FilterValues(
	func(x, y interface{}) bool {
		_, xok := x.(proto.Message)
		_, yok := y.(proto.Message)
		return xok && yok
	},
	cmp.Comparer(func(x, y interface{}) bool {
		vx, ok := x.(proto.Message)
		if !ok {
			return false
		}
		vy, ok := y.(proto.Message)
		if !ok {
			return false
		}
		return proto.Equal(vx, vy)
	}),
)
