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

package sys

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

var (
	inUserNS bool
	nsOnce   sync.Once
)

// RunningInUserNS detects whether we are currently running in a user namespace.
// Originally copied from github.com/lxc/lxd/shared/util.go
func RunningInUserNS() bool {
	nsOnce.Do(func() {
		file, err := os.Open("/proc/self/uid_map")
		if err != nil {
			// This kernel-provided file only exists if user namespaces are supported
			return
		}
		defer file.Close()

		buf := bufio.NewReader(file)
		l, _, err := buf.ReadLine()
		if err != nil {
			return
		}

		line := string(l)
		var a, b, c int64
		fmt.Sscanf(line, "%d %d %d", &a, &b, &c)

		/*
		 * We assume we are in the initial user namespace if we have a full
		 * range - 4294967295 uids starting at uid 0.
		 */
		if a == 0 && b == 0 && c == 4294967295 {
			return
		}
		inUserNS = true
	})
	return inUserNS
}
