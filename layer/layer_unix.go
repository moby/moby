//go:build linux || freebsd || darwin || openbsd

package layer

import "github.com/docker/docker/pkg/stringid"

func (ls *layerStore) mountID(_ string) string {
	return stringid.GenerateRandomID()
}
