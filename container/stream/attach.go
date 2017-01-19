package stream

import (
	"io"
	"sync"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/promise"
)

// DetachError is special error which returned in case of container detach.
type DetachError struct{}

func (DetachError) Error() string {
	return "detached from container"
}

// AttachConfig is the config struct used to attach a client to a stream's stdio
type AttachConfig struct {
	// Tells the attach copier that the stream's stdin is a TTY and to look for
	// escape sequences in stdin to detach from the stream.
	// When true the escape sequence is not passed to the underlying stream
	TTY bool
	// Specifies the detach keys the client will be using
	// Only useful when `TTY` is true
	DetachKeys []byte

	// CloseStdin signals that once done, stdin for the attached stream should be closed
	// For example, this would close the attached container's stdin.
	CloseStdin bool

	// Provide client streams to wire up to
	Stdin          io.ReadCloser
	Stdout, Stderr io.Writer
}

// Attach attaches the stream config to the streams specified in
// the AttachOptions
func (c *Config) Attach(ctx context.Context, cfg *AttachConfig) chan error {
	var (
		cStdout, cStderr io.ReadCloser
		cStdin           io.WriteCloser
		wg               sync.WaitGroup
		errors           = make(chan error, 3)
	)

	if cfg.Stdin != nil {
		cStdin = c.StdinPipe()
		wg.Add(1)
	}

	if cfg.Stdout != nil {
		cStdout = c.StdoutPipe()
		wg.Add(1)
	}

	if cfg.Stderr != nil {
		cStderr = c.StderrPipe()
		wg.Add(1)
	}

	// Connect stdin of container to the attach stdin stream.
	go func() {
		if cfg.Stdin == nil {
			return
		}
		logrus.Debug("attach: stdin: begin")

		var err error
		if cfg.TTY {
			_, err = copyEscapable(cStdin, cfg.Stdin, cfg.DetachKeys)
		} else {
			_, err = io.Copy(cStdin, cfg.Stdin)
		}
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			logrus.Errorf("attach: stdin: %s", err)
			errors <- err
		}
		if cfg.CloseStdin && !cfg.TTY {
			cStdin.Close()
		} else {
			// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
			if cStdout != nil {
				cStdout.Close()
			}
			if cStderr != nil {
				cStderr.Close()
			}
		}
		logrus.Debug("attach: stdin: end")
		wg.Done()
	}()

	attachStream := func(name string, stream io.Writer, streamPipe io.ReadCloser) {
		if stream == nil {
			return
		}

		logrus.Debugf("attach: %s: begin", name)
		_, err := io.Copy(stream, streamPipe)
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			logrus.Errorf("attach: %s: %v", name, err)
			errors <- err
		}
		// Make sure stdin gets closed
		if cfg.Stdin != nil {
			cfg.Stdin.Close()
		}
		streamPipe.Close()
		logrus.Debugf("attach: %s: end", name)
		wg.Done()
	}

	go attachStream("stdout", cfg.Stdout, cStdout)
	go attachStream("stderr", cfg.Stderr, cStderr)

	return promise.Go(func() error {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			// close all pipes
			if cStdin != nil {
				cStdin.Close()
			}
			if cStdout != nil {
				cStdout.Close()
			}
			if cStderr != nil {
				cStderr.Close()
			}
			<-done
		}
		close(errors)
		for err := range errors {
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Code c/c from io.Copy() modified to handle escape sequence
func copyEscapable(dst io.Writer, src io.ReadCloser, keys []byte) (written int64, err error) {
	if len(keys) == 0 {
		// Default keys : ctrl-p ctrl-q
		keys = []byte{16, 17}
	}
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			// ---- Docker addition
			preservBuf := []byte{}
			for i, key := range keys {
				preservBuf = append(preservBuf, buf[0:nr]...)
				if nr != 1 || buf[0] != key {
					break
				}
				if i == len(keys)-1 {
					src.Close()
					return 0, DetachError{}
				}
				nr, er = src.Read(buf)
			}
			var nw int
			var ew error
			if len(preservBuf) > 0 {
				nw, ew = dst.Write(preservBuf)
				nr = len(preservBuf)
			} else {
				// ---- End of docker
				nw, ew = dst.Write(buf[0:nr])
			}
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}
