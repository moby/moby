package streams

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/docker/docker/api/types/streams"
	"github.com/docker/docker/pkg/ioutils"
)

func Open(ctx context.Context, s *streams.Stream) (io.ReadCloser, io.WriteCloser, error) {
	switch s.Spec.Protocol {
	case streams.ProtocolPipe:
		return openPipe(s.Spec.PipeConfig.Path)
	case streams.ProtocolUnixConnect:
		var d net.Dialer
		conn, err := d.DialContext(ctx, "unix", s.Spec.UnixConnectConfig.Addr)
		if err != nil {
			return nil, nil, err
		}

		uc := conn.(*net.UnixConn)
		r := ioutils.NewReadCloserWrapper(uc, uc.CloseRead)
		w := ioutils.NewWriteCloserWrapper(uc, uc.CloseWrite)

		return r, w, nil
	case streams.ProtocolTCPConnect:
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", s.Spec.TCPConnectConfig.Addr+":"+strconv.Itoa(s.Spec.TCPConnectConfig.Port))
		if err != nil {
			return nil, nil, err
		}

		tc := conn.(*net.TCPConn)
		if err := tc.SetKeepAlive(true); err != nil {
			return nil, nil, err
		}
		r := ioutils.NewReadCloserWrapper(tc, tc.CloseRead)
		w := ioutils.NewWriteCloserWrapper(tc, tc.CloseWrite)

		return r, w, err
	}
	return nil, nil, fmt.Errorf("unknown protocol")
}
