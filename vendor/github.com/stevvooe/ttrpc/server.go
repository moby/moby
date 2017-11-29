package ttrpc

import (
	"context"
	"net"

	"github.com/containerd/containerd/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	services *serviceSet
	codec    codec
}

func NewServer() *Server {
	return &Server{
		services: newServiceSet(),
	}
}

func (s *Server) Register(name string, methods map[string]Method) {
	s.services.register(name, methods)
}

func (s *Server) Shutdown(ctx context.Context) error {
	// TODO(stevvooe): Wait on connection shutdown.
	return nil
}

func (s *Server) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.L.WithError(err).Error("failed accept")
			continue
		}

		go s.handleConn(conn)
	}

	return nil
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	type (
		request struct {
			id  uint32
			req *Request
		}

		response struct {
			id   uint32
			resp *Response
		}
	)

	var (
		ch          = newChannel(conn, conn)
		ctx, cancel = context.WithCancel(context.Background())
		responses   = make(chan response)
		requests    = make(chan request)
		recvErr     = make(chan error, 1)
		done        = make(chan struct{})
	)

	defer cancel()
	defer close(done)

	go func() {
		defer close(recvErr)
		var p [messageLengthMax]byte
		for {
			mh, err := ch.recv(ctx, p[:])
			if err != nil {
				recvErr <- err
				return
			}

			if mh.Type != messageTypeRequest {
				// we must ignore this for future compat.
				continue
			}

			var req Request
			if err := s.codec.Unmarshal(p[:mh.Length], &req); err != nil {
				recvErr <- err
				return
			}

			if mh.StreamID%2 != 1 {
				// enforce odd client initiated identifiers.
				select {
				case responses <- response{
					// even though we've had an invalid stream id, we send it
					// back on the same stream id so the client knows which
					// stream id was bad.
					id: mh.StreamID,
					resp: &Response{
						Status: status.New(codes.InvalidArgument, "StreamID must be odd for client initiated streams").Proto(),
					},
				}:
				case <-done:
				}

				continue
			}

			select {
			case requests <- request{
				id:  mh.StreamID,
				req: &req,
			}:
			case <-done:
			}
		}
	}()

	for {
		select {
		case request := <-requests:
			go func(id uint32) {
				p, status := s.services.call(ctx, request.req.Service, request.req.Method, request.req.Payload)
				resp := &Response{
					Status:  status.Proto(),
					Payload: p,
				}

				select {
				case responses <- response{
					id:   id,
					resp: resp,
				}:
				case <-done:
				}
			}(request.id)
		case response := <-responses:
			p, err := s.codec.Marshal(response.resp)
			if err != nil {
				log.L.WithError(err).Error("failed marshaling response")
				return
			}
			if err := ch.send(ctx, response.id, messageTypeResponse, p); err != nil {
				log.L.WithError(err).Error("failed sending message on channel")
				return
			}
		case err := <-recvErr:
			log.L.WithError(err).Error("error receiving message")
			return
		}
	}
}
