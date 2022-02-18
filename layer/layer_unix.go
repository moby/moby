//go:build linux || freebsd || darwin || openbsd
// +build linux freebsd darwin openbsd

package layer // import "github.com/moby/moby/layer"

import "github.com/moby/moby/pkg/stringid"

func (ls *layerStore) mountID(name string) string {
	return stringid.GenerateRandomID()
}
