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

package platforms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/osversion"
	"golang.org/x/sys/windows"
)

func getWindowsOsVersion() string {
	major, minor, build := windows.RtlGetNtVersionNumbers()
	return fmt.Sprintf("%d.%d.%d", major, minor, build)
}

// Deprecated: this function is deprecated, and removed in github.com/containerd/platforms
func GetOsVersion(osVersionPrefix string) osversion.OSVersion {
	parts := strings.Split(osVersionPrefix, ".")
	if len(parts) < 3 {
		return osversion.OSVersion{}
	}

	majorVersion, _ := strconv.Atoi(parts[0])
	minorVersion, _ := strconv.Atoi(parts[1])
	buildNumber, _ := strconv.Atoi(parts[2])

	return osversion.OSVersion{
		MajorVersion: uint8(majorVersion),
		MinorVersion: uint8(minorVersion),
		Build:        uint16(buildNumber),
	}
}
