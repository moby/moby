package osversion

// List of stable ABI compliant ltsc releases
// Note: List must be sorted in ascending order
var compatLTSCReleases = []uint16{
	LTSC2022,
	LTSC2025,
}

// CheckHostAndContainerCompat checks if given host and container
// OS versions are compatible.
// It includes support for stable ABI compliant versions as well.
// Every release after WS 2022 will support the previous ltsc
// container image. Stable ABI is in preview mode for windows 11 client.
// Refer: https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility?tabs=windows-server-2022%2Cwindows-10#windows-server-host-os-compatibility
func CheckHostAndContainerCompat(host, ctr OSVersion) bool {
	// check major minor versions of host and guest
	if host.MajorVersion != ctr.MajorVersion ||
		host.MinorVersion != ctr.MinorVersion {
		return false
	}

	// If host is < WS 2022, exact version match is required
	if host.Build < LTSC2022 {
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
	var supportedLTSCRelease uint16 = LTSC2022
	for i := len(compatLTSCReleases) - 1; i >= 0; i-- {
		if host.Build > compatLTSCReleases[i] {
			supportedLTSCRelease = compatLTSCReleases[i]
			break
		}
	}
	return supportedLTSCRelease <= ctr.Build && ctr.Build <= host.Build
}
