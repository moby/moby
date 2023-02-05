package streams

import (
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/docker/docker/api/types/streams"
)

func Open(s *streams.Stream) (io.ReadCloser, io.WriteCloser, error) {
	switch s.Spec.Protocol {
	case streams.ProtocolPipe:
		return openPipe(s.Spec.PipeConfig.Path)
	case streams.ProtocolUnixConnect:
		conn, err := net.Dial("unix", s.Spec.UnixConnectConfig.Addr)
		return conn, conn, err
	case streams.ProtocolTCPConnect:
		conn, err := net.Dial("tcp", s.Spec.TCPConnectConfig.Addr+":"+strconv.Itoa(s.Spec.TCPConnectConfig.Port))
		return conn, conn, err
	}
	return nil, nil, fmt.Errorf("unknown protocol")
}
