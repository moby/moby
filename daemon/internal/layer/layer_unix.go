//go:build linux || freebsd || darwin || openbsd

package layer

import "github.com/moby/moby/daemon/pkg/stringid"

func (ls *layerStore) mountID(name string) string {
	return stringid.GenerateRandomID()
}
