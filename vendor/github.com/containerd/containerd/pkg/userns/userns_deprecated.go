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

// Deprecated: use github.com/moby/sys/userns
package userns

import "github.com/moby/sys/userns"

// RunningInUserNS detects whether we are currently running in a Linux
// user namespace and memoizes the result. It returns false on non-Linux
// platforms.
//
// Deprecated: use [userns.RunningInUserNS].
func RunningInUserNS() bool {
	return userns.RunningInUserNS()
}
