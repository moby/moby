package docker

import (
	"errors"
	"net"
	"os"
)

var sdNotifyNoSocket = errors.New("No socket")

// Send a message to the init daemon. It is common to ignore the error.
func sd_notify(state string) error {
	socketAddr := &net.UnixAddr{
		Name: os.Getenv("NOTIFY_SOCKET"),
		Net:  "unixgram",
	}

	if socketAddr.Name == "" {
		return sdNotifyNoSocket
	}

	conn, err := net.DialUnix(socketAddr.Net, nil, socketAddr)
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte(state))
	if err != nil {
		return err
	}

	return nil
}
