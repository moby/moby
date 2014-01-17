package activation

import (
	"fmt"
	"net"
)

// Listeners returns net.Listeners for all socket activated fds passed to this process.
func Listeners(unsetEnv bool) ([]net.Listener, error) {
	files := Files(unsetEnv)
	listeners := make([]net.Listener, len(files))

	for i, f := range files {
		var err error
		listeners[i], err = net.FileListener(f)
		if err != nil {
			return nil, fmt.Errorf("Error setting up FileListener for fd %d: %s", f.Fd(), err.Error())
		}
	}

	return listeners, nil
}
