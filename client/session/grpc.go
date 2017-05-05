package session

import (
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
)

var once sync.Once

func serve(ctx context.Context, grpcServer *grpc.Server, conn net.Conn) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	logrus.Debugf("serving grpc connection")
	(&http2.Server{}).ServeConn(conn, &http2.ServeConnOpts{Handler: grpcServer})
}

type grpcCaller struct {
	cc *grpc.ClientConn
}

func newCaller(ctx context.Context, conn net.Conn) (*grpcCaller, error) {
	dialOpt := grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
		return conn, nil
	})

	cc, err := grpc.DialContext(ctx, "", dialOpt, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create grpc client")
	}

	gc := &grpcCaller{
		cc: cc,
	}

	go func() {
		<-ctx.Done()
		cc.Close()
	}()

	return gc, nil
}
