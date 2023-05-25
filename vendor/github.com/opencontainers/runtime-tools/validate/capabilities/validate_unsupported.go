//go:build !linux
// +build !linux

package capabilities

import (
	"github.com/syndtr/gocapability/capability"
)

// LastCap return last cap of system
func LastCap() capability.Cap {
	return capability.Cap(-1)
}
