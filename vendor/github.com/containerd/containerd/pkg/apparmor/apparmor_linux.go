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

package apparmor

import (
	"os"
	"sync"
)

var (
	appArmorSupported bool
	checkAppArmor     sync.Once
)

// hostSupports returns true if apparmor is enabled for the host, if
// apparmor_parser is enabled, and if we are not running docker-in-docker.
//
// It is a modified version of libcontainer/apparmor.IsEnabled(), which does not
// check for apparmor_parser to be present, or if we're running docker-in-docker.
func hostSupports() bool {
	checkAppArmor.Do(func() {
		// see https://github.com/opencontainers/runc/blob/0d49470392206f40eaab3b2190a57fe7bb3df458/libcontainer/apparmor/apparmor_linux.go
		if _, err := os.Stat("/sys/kernel/security/apparmor"); err == nil && os.Getenv("container") == "" {
			buf, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
			appArmorSupported = err == nil && len(buf) > 1 && buf[0] == 'Y'
		}
	})
	return appArmorSupported
}
