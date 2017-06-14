// +build linux freebsd darwin openbsd solaris

package layer

import "github.com/docker/docker/pkg/stringid"

// RegisterOpts contain optional arguments to be passed to Register
type RegisterOpts struct {
}

func (ls *layerStore) mountID(name string) string {
	return stringid.GenerateRandomID()
}
