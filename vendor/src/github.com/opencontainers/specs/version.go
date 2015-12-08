package specs

import "fmt"

const (
	// VersionMajor is for an API incompatible changes
	VersionMajor = 0
	// VersionMinor is for functionality in a backwards-compatible manner
	VersionMinor = 2
	// VersionPatch is for backwards-compatible bug fixes
	VersionPatch = 0
)

// Version is the specification version that the package types support.
var Version = fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)
