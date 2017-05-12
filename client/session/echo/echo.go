package echo

import (
	"context"
	"io"

	"github.com/docker/docker/client/session"
	"github.com/docker/docker/client/session/echo/internal"
)

type ServerMessageInterceptor func(string)

type echoServer struct {
	messages chan<- string
}

func (s *echoServer) MessageStream(streamSrv internal.Echo_MessageStreamServer) error {
	for {
		value, err := streamSrv.Recv()
		if err == io.EOF {
			close(s.messages)
			return nil
		}
		if err != nil {
			close(s.messages)
			return err
		}
		s.messages <- value.Text
		err = streamSrv.Send(value)
		if err != nil {
			close(s.messages)
			return err
		}
	}
}
func AttachEchoServerToSession(sess *session.ServerSession, name string, messages chan<- string) {
	svcDesc := *internal.EchoServiceDesc()
	svcDesc.ServiceName = name
	sess.Allow(&svcDesc, &echoServer{messages: messages})
}

func TrySetupEchoClient(ctx context.Context, sess session.Caller, name string, messageStream <-chan string, receivedStream chan<- string) (bool, error) {
	if !sess.Supports(name) {
		return false, nil
	}

	client := internal.NewNamedEchoClient(sess.GetGrpcConn(), name)
	stream, err := client.MessageStream(ctx)
	if err != nil {
		return false, err
	}

	go func() {
		for m := range messageStream {
			stream.Send(&internal.Value{Text: m})
		}
		stream.CloseSend()
	}()

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				break
			}
			receivedStream <- msg.Text
		}
		close(receivedStream)
	}()

	return true, nil
}
