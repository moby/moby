package request // import "github.com/docker/docker/integration-cli/request"

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

func npipeDial(path string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(path, &timeout)
}
