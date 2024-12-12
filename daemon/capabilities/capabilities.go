package capabilities

// CapabilitiesVersion is the current version of the Capabilities API.
//
// If a client requests a version N such that N > CapabilitiesVersion
// with this version. Otherwise, the engine will respond, downgrading
// if necessary, with the requested version.
const CurrentVersion = V1

type Version int

const (
	_ Version = iota
	V1
)

type Capabilities = capabilitiesV1

// VersionedCapabilities is the general interface used for Capabilities.
// All types representing different capabilities versions should implement
// VersionedCapabilities.
type VersionedCapabilities interface {
	Version() Version
}

// CapabilitiesBase is the base Capabilities type.
// It should not be used directly, but instead embedded into versioned
// Capabilities types.
type CapabilitiesBase struct {
	// CapabilitiesVersion is a natural number representing the version
	// of the capabilities API this response is for.
	//
	// This is part of the Capabilities version negotiation flow:
	// Since the API contract states that any client requesting
	// capabilities version N MUST also accept any capabilities
	// version M such that 1 < M < N, the daemon can pick a version
	// M = min(DAEMON_CAPABILITIES_VERSION, N), and send back a
	// response for that version.
	CapabilitiesVersion Version `json:"_v"`
}

func (b CapabilitiesBase) Version() Version {
	return b.CapabilitiesVersion
}

func negotiatedCapabilitiesVersion(requestVersion int) Version {
	if requestVersion < int(CurrentVersion) && requestVersion > 0 {
		return Version(requestVersion)
	}

	return CurrentVersion
}
