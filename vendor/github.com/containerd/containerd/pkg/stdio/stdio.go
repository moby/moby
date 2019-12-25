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

package stdio

// Stdio of a process
type Stdio struct {
	Stdin    string
	Stdout   string
	Stderr   string
	Terminal bool
}

// IsNull returns true if the stdio is not defined
func (s Stdio) IsNull() bool {
	return s.Stdin == "" && s.Stdout == "" && s.Stderr == ""
}
