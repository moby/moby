// +build windows

package opts

import (
	"os/user"
)

// DefaultHost constant defines the default host string used by docker on Windows
var DefaultHost = "npipe://" + DefaultNamedPipe

func whoami() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return usr.Username, nil
}
