package system

// CapabilitiesVersion is the current version of the Capabilities API.
//
// If a client requests a version N such that N > CapabilitiesVersion
// with this version. Otherwise, the engine will respond, downgrading
// if necessary, with the requested version.
const CapabilitiesVersion = 1

type Capabilities struct {
	// Version is a natural number representing the version of the
	// capabilities API this response is for.
	//
	// This is part of the capabilities version negotiation flow:
	// Since the API contract states that any client requesting
	// capabilities version N MUST also accept any capabilities
	// version M such that 1 < M < N, the daemon can pick a version
	// M = min(DAEMON_CAPABILITIES_VERSION, N), and send back a
	// response for that version.
	Version int `json:"_v"`

	RegistryClientAuth bool `json:"registry-client-auth"`
}
