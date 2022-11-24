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
	"github.com/containerd/typeurl/v2"
	"google.golang.org/protobuf/types/known/anypb"
)

// FromAny converts typeurl.Any to github.com/containerd/containerd/protobuf/types.Any.
func FromAny(from typeurl.Any) *anypb.Any {
	if from == nil {
		return nil
	}

	if pbany, ok := from.(*anypb.Any); ok {
		return pbany
	}

	return &anypb.Any{
		TypeUrl: from.GetTypeUrl(),
		Value:   from.GetValue(),
	}
}

// FromAny converts an arbitrary interface to github.com/containerd/containerd/protobuf/types.Any.
func MarshalAnyToProto(from interface{}) (*anypb.Any, error) {
	any, err := typeurl.MarshalAny(from)
	if err != nil {
		return nil, err
	}
	return FromAny(any), nil
}
