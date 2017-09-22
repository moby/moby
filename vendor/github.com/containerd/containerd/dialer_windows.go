package containerd

import (
	"net"
	"os"
	"syscall"
	"time"

	winio "github.com/Microsoft/go-winio"
)

func isNoent(err error) bool {
	if err != nil {
		if oerr, ok := err.(*os.PathError); ok {
			if oerr.Err == syscall.ENOENT {
				return true
			}
		}
	}
	return false
}

func dialer(address string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(address, &timeout)
}

func DialAddress(address string) string {
	return address
}
