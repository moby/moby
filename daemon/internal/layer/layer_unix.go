//go:build linux || freebsd || darwin || openbsd

package layer

import "github.com/docker/docker/daemon/internal/stringid"

func (ls *layerStore) mountID(name string) string {
	return stringid.GenerateRandomID()
}
