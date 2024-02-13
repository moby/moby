package capabilities

import (
	"github.com/syndtr/gocapability/capability"
)

// LastCap return last cap of system
func LastCap() capability.Cap {
	last := capability.CAP_LAST_CAP
	// hack for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}

	return last
}
