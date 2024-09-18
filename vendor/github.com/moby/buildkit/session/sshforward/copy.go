package sshforward

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Stream interface {
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

func Copy(ctx context.Context, conn io.ReadWriteCloser, stream Stream, closeStream func() error) error {
	defer conn.Close()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() (retErr error) {
		p := &BytesMessage{}
		for {
			if err := stream.RecvMsg(p); err != nil {
				if err == io.EOF {
					// indicates client performed CloseSend, but they may still be
					// reading data
					if closeWriter, ok := conn.(interface {
						CloseWrite() error
					}); ok {
						closeWriter.CloseWrite()
					} else {
						conn.Close()
					}
					return nil
				}
				conn.Close()
				return errors.WithStack(err)
			}
			select {
			case <-ctx.Done():
				conn.Close()
				return context.Cause(ctx)
			default:
			}
			if _, err := conn.Write(p.Data); err != nil {
				conn.Close()
				return errors.WithStack(err)
			}
			p.Data = p.Data[:0]
		}
	})

	g.Go(func() (retErr error) {
		for {
			buf := make([]byte, 32*1024)
			n, err := conn.Read(buf)
			switch {
			case err == io.EOF:
				if closeStream != nil {
					closeStream()
				}
				return nil
			case err != nil:
				return errors.WithStack(err)
			}
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			default:
			}
			p := &BytesMessage{Data: buf[:n]}
			if err := stream.SendMsg(p); err != nil {
				return errors.WithStack(err)
			}
		}
	})

	return g.Wait()
}
