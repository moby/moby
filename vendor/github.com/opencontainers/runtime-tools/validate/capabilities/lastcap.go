package capabilities

import (
	"github.com/moby/sys/capability"
)

// LastCap returns last cap of system.
//
// Deprecated: use github.com/moby/sys/capability.LastCap instead.
func LastCap() capability.Cap {
	last, err := capability.LastCap()
	if err != nil {
		return -1
	}
	return last
}
