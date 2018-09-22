package grpchijack

import (
	"net"

	controlapi "github.com/moby/buildkit/api/services/control"
	"google.golang.org/grpc/metadata"
)

// Hijack hijacks session to a connection.
func Hijack(stream controlapi.Control_SessionServer) (net.Conn, <-chan struct{}, map[string][]string) {
	md, _ := metadata.FromIncomingContext(stream.Context())
	c, closeCh := streamToConn(stream)
	return c, closeCh, md
}
