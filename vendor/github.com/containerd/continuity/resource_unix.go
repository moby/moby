// +build linux darwin freebsd solaris

package continuity

import (
	"fmt"
	"os"
	"syscall"
)

// newBaseResource returns a *resource, populated with data from p and fi,
// where p will be populated directly.
func newBaseResource(p string, fi os.FileInfo) (*resource, error) {
	// TODO(stevvooe): This need to be resolved for the container's root,
	// where here we are really getting the host OS's value. We need to allow
	// this be passed in and fixed up to make these uid/gid mappings portable.
	// Either this can be part of the driver or we can achieve it through some
	// other mechanism.
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// TODO(stevvooe): This may not be a hard error for all platforms. We
		// may want to move this to the driver.
		return nil, fmt.Errorf("unable to resolve syscall.Stat_t from (os.FileInfo).Sys(): %#v", fi)
	}

	return &resource{
		paths: []string{p},
		mode:  fi.Mode(),

		uid: int64(sys.Uid),
		gid: int64(sys.Gid),

		// NOTE(stevvooe): Population of shared xattrs field is deferred to
		// the resource types that populate it. Since they are a property of
		// the context, they must set there.
	}, nil
}
