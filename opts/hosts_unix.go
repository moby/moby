// +build !windows

package opts

import (
	"fmt"
	"os"

	"github.com/docker/docker/pkg/idtools"
)

// DefaultHost constant defines the default host string used by docker on other hosts than Windows
var DefaultHost = fmt.Sprintf("unix://%s", DefaultUnixSocket)

func whoami() (string, error) {
	// on Unix, we avoid to call os/user.Current(), because a call to os/user.Current()
	// in a static binary leads to segfault due to a glibc issue that won't be fixed in a short term.
	// (#29344, golang/go#13470, https://sourceware.org/bugzilla/show_bug.cgi?id=19341)
	usr, err := idtools.LookupUID(os.Getuid())
	if err != nil {
		return "", err
	}
	return usr.Name, nil
}
