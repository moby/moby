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
	"strconv"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// windowsOSVersion is a wrapper for Windows version information
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724439(v=vs.85).aspx
type windowsOSVersion struct {
	Version      uint32
	MajorVersion uint8
	MinorVersion uint8
	Build        uint16
}

// Windows Client and Server build numbers.
//
// See:
// https://learn.microsoft.com/en-us/windows/release-health/release-information
// https://learn.microsoft.com/en-us/windows/release-health/windows-server-release-info
// https://learn.microsoft.com/en-us/windows/release-health/windows11-release-information
const (
	// rs5 (version 1809, codename "Redstone 5") corresponds to Windows Server
	// 2019 (ltsc2019), and Windows 10 (October 2018 Update).
	rs5 = 17763
	// ltsc2019 (Windows Server 2019) is an alias for [RS5].
	ltsc2019 = rs5

	// v21H2Server corresponds to Windows Server 2022 (ltsc2022).
	v21H2Server = 20348
	// ltsc2022 (Windows Server 2022) is an alias for [v21H2Server]
	ltsc2022 = v21H2Server

	// v22H2Win11 corresponds to Windows 11 (2022 Update).
	v22H2Win11 = 22621

	// v23H2 is the 23H2 release in the Windows Server annual channel.
	v23H2 = 25398

	// Windows Server 2025 build 26100
	v25H1Server = 26100
	ltsc2025    = v25H1Server
)

// List of stable ABI compliant ltsc releases
// Note: List must be sorted in ascending order
var compatLTSCReleases = []uint16{
	ltsc2022,
	ltsc2025,
}

// CheckHostAndContainerCompat checks if given host and container
// OS versions are compatible.
// It includes support for stable ABI compliant versions as well.
// Every release after WS 2022 will support the previous ltsc
// container image. Stable ABI is in preview mode for windows 11 client.
// Refer: https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility?tabs=windows-server-2022%2Cwindows-10#windows-server-host-os-compatibility
func checkWindowsHostAndContainerCompat(host, ctr windowsOSVersion) bool {
	// check major minor versions of host and guest
	if host.MajorVersion != ctr.MajorVersion ||
		host.MinorVersion != ctr.MinorVersion {
		return false
	}

	// If host is < WS 2022, exact version match is required
	if host.Build < ltsc2022 {
		return host.Build == ctr.Build
	}

	// Find the latest LTSC version that is earlier than the host version.
	// This is the earliest version of container that the host can run.
	//
	// If the host version is an LTSC, then it supports compatibility with
	// everything from the previous LTSC up to itself, so we want supportedLTSCRelease
	// to be the previous entry.
	//
	// If no match is found, then we know that the host is LTSC2022 exactly,
	// since we already checked that it's not less than LTSC2022.
	var supportedLTSCRelease uint16 = ltsc2022
	for i := len(compatLTSCReleases) - 1; i >= 0; i-- {
		if host.Build > compatLTSCReleases[i] {
			supportedLTSCRelease = compatLTSCReleases[i]
			break
		}
	}
	return supportedLTSCRelease <= ctr.Build && ctr.Build <= host.Build
}

func getWindowsOSVersion(osVersionPrefix string) windowsOSVersion {
	if strings.Count(osVersionPrefix, ".") < 2 {
		return windowsOSVersion{}
	}

	major, extra, _ := strings.Cut(osVersionPrefix, ".")
	minor, extra, _ := strings.Cut(extra, ".")
	build, _, _ := strings.Cut(extra, ".")

	majorVersion, err := strconv.ParseUint(major, 10, 8)
	if err != nil {
		return windowsOSVersion{}
	}

	minorVersion, err := strconv.ParseUint(minor, 10, 8)
	if err != nil {
		return windowsOSVersion{}
	}
	buildNumber, err := strconv.ParseUint(build, 10, 16)
	if err != nil {
		return windowsOSVersion{}
	}

	return windowsOSVersion{
		MajorVersion: uint8(majorVersion),
		MinorVersion: uint8(minorVersion),
		Build:        uint16(buildNumber),
	}
}

type windowsVersionMatcher struct {
	windowsOSVersion
}

func (m windowsVersionMatcher) Match(v string) bool {
	if m.isEmpty() || v == "" {
		return true
	}
	osv := getWindowsOSVersion(v)
	return checkWindowsHostAndContainerCompat(m.windowsOSVersion, osv)
}

func (m windowsVersionMatcher) isEmpty() bool {
	return m.MajorVersion == 0 && m.MinorVersion == 0 && m.Build == 0
}

type windowsMatchComparer struct {
	Matcher
}

func (c *windowsMatchComparer) Less(p1, p2 specs.Platform) bool {
	m1, m2 := c.Match(p1), c.Match(p2)
	if m1 && m2 {
		return p1.OSVersion > p2.OSVersion
	}
	return m1 && !m2
}
