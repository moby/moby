package sshforward

import (
	io "io"

	"github.com/pkg/errors"
	context "golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

type Stream interface {
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

func Copy(ctx context.Context, conn io.ReadWriteCloser, stream Stream, closeStream func() error) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() (retErr error) {
		p := &BytesMessage{}
		for {
			if err := stream.RecvMsg(p); err != nil {
				conn.Close()
				if err == io.EOF {
					return nil
				}
				return errors.WithStack(err)
			}
			select {
			case <-ctx.Done():
				conn.Close()
				return ctx.Err()
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
				return ctx.Err()
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
