//go:build !windows

package request

import (
	"net"
	"time"
)

func npipeDial(_ string, _ time.Duration) (net.Conn, error) {
	panic("npipe protocol only supported on Windows")
}
