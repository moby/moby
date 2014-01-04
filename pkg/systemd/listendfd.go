package systemd

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/coreos/go-systemd/activation"
)

// ListenFD returns the specified socket activated files as a slice of
// net.Listeners or all of the activated files if "*" is given.
func ListenFD(addr string) ([]net.Listener, error) {
	files := activation.Files(false)
	if files == nil || len(files) == 0 {
		return nil, errors.New("No sockets found")
	}

	// default to all fds just like unix:// and tcp://
	if addr == "" {
		addr = "*"
	}

	fdNum, _ := strconv.Atoi(addr)
	fdOffset := fdNum - 3
	if (addr != "*") && (len(files) < int(fdOffset)+1) {
		return nil, errors.New("Too few socket activated files passed in")
	}

	// socket activation
	listeners := make([]net.Listener, len(files))
	for i, f := range files {
		var err error
		listeners[i], err = net.FileListener(f)
		if err != nil {
			return nil, fmt.Errorf("Error setting up FileListener for fd %d: %s", f.Fd(), err.Error())
		}
	}

	if addr == "*" {
		return listeners, nil
	}

	return []net.Listener{listeners[fdOffset]}, nil
}
