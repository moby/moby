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

package filters

// Adaptor specifies the mapping of fieldpaths to a type. For the given field
// path, the value and whether it is present should be returned. The mapping of
// the fieldpath to a field is deferred to the adaptor implementation, but
// should generally follow protobuf field path/mask semantics.
type Adaptor interface {
	Field(fieldpath []string) (value string, present bool)
}

// AdapterFunc allows implementation specific matching of fieldpaths
type AdapterFunc func(fieldpath []string) (string, bool)

// Field returns the field name and true if it exists
func (fn AdapterFunc) Field(fieldpath []string) (string, bool) {
	return fn(fieldpath)
}
