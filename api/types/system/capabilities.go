package system

import (
	"fmt"
)

// CapabilitiesVersion is the current version of the Capabilities API.
//
// If a client requests a version N such that N > CapabilitiesVersion
// with this version. Otherwise, the engine will respond, downgrading
// if necessary, with the requested version.
const CurrentVersion = 1

// Capabilities represents a specific version of Capabilities.
type Capabilities struct {
	// Version is a natural number representing the version of the
	// capabilities API this response is for.
	// The version of the Capabilities is intrinsically tied to
	// the values contained in Data, and necessary to parse them.
	//
	// The capabilities API contract mandates that any client requesting
	// capabilities version N MUST also accept any capabilities version
	// M such that 1 < M < N. When handling a capabilities request for
	// version N, the daemon will pick a version
	// M = min(DAEMON_CAPABILITIES_VERSION, N) and reply with that
	// version.
	Version int `json:"_v"`

	// Data contains the internal representation of the advertised
	// capabilities.
	// Values contained therein should not be processed without
	// taking Version into account, and no inferences about the
	// capabilities of the engine should be made solely on it.
	Data map[string]any `json:"data"`
}

func (b Capabilities) CapabilitiesVersion() int {
	return b.Version
}

type ErrBadCapabilities struct {
	cause error
}

func (e ErrBadCapabilities) Error() string {
	return "bad capabilities: " + e.cause.Error()
}

type ErrUnknownCapabilitiesVersion struct {
	version int
}

func (e ErrUnknownCapabilitiesVersion) Error() string {
	return fmt.Sprintf("can't process unknown capabilities version %d", e.version)
}

func (b Capabilities) SupportsRegistryClientAuth() (bool, error) {
	switch b.CapabilitiesVersion() {
	case 1:
		v, ok := b.Data["registry-client-auth"]
		if !ok {
			return false, ErrBadCapabilities{
				cause: fmt.Errorf("capabilities version %d must contain `registry-client-auth`", b.CapabilitiesVersion()),
			}
		}
		if b, ok := v.(bool); ok {
			return b, nil
		}
	}

	return false, ErrUnknownCapabilitiesVersion{version: b.CapabilitiesVersion()}
}
