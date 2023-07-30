package remote // import "github.com/docker/docker/libcontainerd/remote"

import (
	"io"
	"net"
	"sync"

	winio "github.com/Microsoft/go-winio"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
)

type delayedConnection struct {
	l    net.Listener
	con  net.Conn
	wg   sync.WaitGroup
	once sync.Once
}

func (dc *delayedConnection) Write(p []byte) (int, error) {
	dc.wg.Wait()
	if dc.con != nil {
		return dc.con.Write(p)
	}
	return 0, errors.New("use of closed network connection")
}

func (dc *delayedConnection) Read(p []byte) (int, error) {
	dc.wg.Wait()
	if dc.con != nil {
		return dc.con.Read(p)
	}
	return 0, errors.New("use of closed network connection")
}

func (dc *delayedConnection) unblockConnectionWaiters() {
	defer dc.once.Do(func() {
		dc.wg.Done()
	})
}

func (dc *delayedConnection) Close() error {
	dc.l.Close()
	if dc.con != nil {
		return dc.con.Close()
	}
	dc.unblockConnectionWaiters()
	return nil
}

type stdioPipes struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// newStdioPipes creates actual fifos for stdio.
func (c *client) newStdioPipes(fifos *cio.FIFOSet) (_ *stdioPipes, err error) {
	p := &stdioPipes{}
	if fifos.Stdin != "" {
		c.logger.WithField("stdin", fifos.Stdin).Debug("listen")
		l, err := winio.ListenPipe(fifos.Stdin, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create stdin pipe %s", fifos.Stdin)
		}
		dc := &delayedConnection{
			l: l,
		}
		dc.wg.Add(1)
		defer func() {
			if err != nil {
				dc.Close()
			}
		}()
		p.stdin = dc

		go func() {
			c.logger.WithField("stdin", fifos.Stdin).Debug("accept")
			conn, err := l.Accept()
			if err != nil {
				dc.Close()
				if err != winio.ErrPipeListenerClosed {
					c.logger.WithError(err).Errorf("failed to accept stdin connection on %s", fifos.Stdin)
				}
				return
			}
			c.logger.WithField("stdin", fifos.Stdin).Debug("connected")
			dc.con = conn
			dc.unblockConnectionWaiters()
		}()
	}

	if fifos.Stdout != "" {
		c.logger.WithField("stdout", fifos.Stdout).Debug("listen")
		l, err := winio.ListenPipe(fifos.Stdout, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create stdout pipe %s", fifos.Stdout)
		}
		dc := &delayedConnection{
			l: l,
		}
		dc.wg.Add(1)
		defer func() {
			if err != nil {
				dc.Close()
			}
		}()
		p.stdout = dc

		go func() {
			c.logger.WithField("stdout", fifos.Stdout).Debug("accept")
			conn, err := l.Accept()
			if err != nil {
				dc.Close()
				if err != winio.ErrPipeListenerClosed {
					c.logger.WithFields(log.Fields{"error": err, "stdout": fifos.Stdout}).Error("failed to accept stdout connection")
				}
				return
			}
			c.logger.WithField("stdout", fifos.Stdout).Debug("connected")
			dc.con = conn
			dc.unblockConnectionWaiters()
		}()
	}

	if fifos.Stderr != "" {
		c.logger.WithField("stderr", fifos.Stderr).Debug("listen")
		l, err := winio.ListenPipe(fifos.Stderr, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create stderr pipe %s", fifos.Stderr)
		}
		dc := &delayedConnection{
			l: l,
		}
		dc.wg.Add(1)
		defer func() {
			if err != nil {
				dc.Close()
			}
		}()
		p.stderr = dc

		go func() {
			c.logger.WithField("stderr", fifos.Stderr).Debug("accept")
			conn, err := l.Accept()
			if err != nil {
				dc.Close()
				if err != winio.ErrPipeListenerClosed {
					c.logger.WithError(err).Errorf("failed to accept stderr connection on %s", fifos.Stderr)
				}
				return
			}
			c.logger.WithField("stderr", fifos.Stderr).Debug("connected")
			dc.con = conn
			dc.unblockConnectionWaiters()
		}()
	}
	return p, nil
}
