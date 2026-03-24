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

package api

import (
	"strings"
)

// ToOCI returns an OCI Env entry for the KeyValue.
func (e *KeyValue) ToOCI() string {
	return e.Key + "=" + e.Value
}

// FromOCIEnv returns KeyValues from an OCI runtime Spec environment.
func FromOCIEnv(in []string) []*KeyValue {
	if in == nil {
		return nil
	}
	out := []*KeyValue{}
	for _, keyval := range in {
		var key, val string
		split := strings.SplitN(keyval, "=", 2)
		switch len(split) {
		case 0:
			continue
		case 1:
			key = split[0]
		case 2:
			key = split[0]
			val = split[1]
		default:
			val = strings.Join(split[1:], "=")
		}
		out = append(out, &KeyValue{
			Key:   key,
			Value: val,
		})
	}
	return out
}

// IsMarkedForRemoval checks if an environment variable is marked for removal.
func (e *KeyValue) IsMarkedForRemoval() (string, bool) {
	key, marked := IsMarkedForRemoval(e.Key)
	return key, marked
}
