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

package continuity

import "os"

type hardlinkKey struct{}

func newHardlinkKey(fi os.FileInfo) (hardlinkKey, error) {
	// NOTE(stevvooe): Obviously, this is not yet implemented. However, the
	// makings of an implementation are available in src/os/types_windows.go. More
	// investigation needs to be done to figure out exactly how to do this.
	return hardlinkKey{}, errNotAHardLink
}
