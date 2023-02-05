package pipes

import (
	"context"
	"fmt"
	"io"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

func NewCopier(ctx context.Context, r *PipeReader, writers ...*PipeWriter) (*Copier, error) {
	ls := make([]syscall.RawConn, 0, len(writers))
	for _, w := range writers {
		wrc, err := w.SyscallConn()
		if err != nil {
			return nil, err
		}
		ls = append(ls, wrc)
	}

	rwc, err := r.SyscallConn()
	if err != nil {
		return nil, err
	}

	var buf [2]int
	if err := unix.Pipe2(buf[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
		return nil, fmt.Errorf("error creating pipe buffer: %w", err)
	}

	c := &Copier{
		r:       rwc,
		writers: ls,
		buf:     buf,
	}

	c.cond = sync.NewCond(&c.mu)

	go c.run(ctx)

	return c, nil
}

type Copier struct {
	r       syscall.RawConn
	writers []syscall.RawConn

	mu        sync.Mutex
	cond      *sync.Cond
	pending   []syscall.RawConn
	closedErr error

	buf [2]int

	// This is used for teseting purposes
	_lastErr error
}

func (c *Copier) run(ctx context.Context) {
	defer func() {
		unix.Close(c.buf[0])
		unix.Close(c.buf[1])
	}()

	for {
		if err := c.wait(ctx); err != nil {
			return
		}

		c.doCopy(ctx)
	}
}

func (c *Copier) setClosedErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err == nil || c.closedErr != nil {
		return
	}

	c.closedErr = err
}

func (c *Copier) Add(w *PipeWriter) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.closedErr; err != nil {
		return err
	}

	wrc, err := w.SyscallConn()
	if err != nil {
		return err
	}

	c.pending = append(c.pending, wrc)
	c.cond.Signal()

	return nil
}

func (c *Copier) lastErr() error {
	c.mu.Lock()
	err := c._lastErr
	c.mu.Unlock()
	return err
}

func (c *Copier) shouldWait(ctx context.Context) bool {
	return len(c.writers) == 0 && len(c.pending) == 0 && c.closedErr == nil && ctx.Err() == nil
}

func (c *Copier) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.shouldWait(ctx) {
		c.cond.Wait()
	}

	if c.closedErr != nil {
		return c.closedErr
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(c.pending) > 0 {
		c.writers = append(c.writers, c.pending...)
		c.pending = c.pending[:0]
	}
	return nil
}

func (c *Copier) doCopy(ctx context.Context) {
	if ctx.Err() != nil {
		c.setClosedErr(ctx.Err())
		return
	}

	var (
		evict []int
	)

	err := c.r.Read(func(rfd uintptr) bool {
		if err := c.wait(ctx); err != nil {
			return true
		}

		var (
			spliced bool
		)

		total, err := splice(int(rfd), c.buf[1], 0)
		if err != nil && err != unix.EAGAIN {
			c.setClosedErr(err)
			return true
		}

		if total == 0 {
			if err == unix.EAGAIN {
				return false
			}
			if err == nil {
				c.setClosedErr(io.EOF)
				return true
			}
		}

		for i, wrc := range c.writers {
			if ctx.Err() != nil {
				c.setClosedErr(ctx.Err())
				return true
			}

			if i == len(c.writers)-1 {
				n, err := c.doSplice(uintptr(c.buf[0]), wrc, total)
				if (err != nil && err != unix.EAGAIN) || (total > 0 && n < total) {
					c.mu.Lock()
					c._lastErr = err
					c.mu.Unlock()
					evict = append(evict, i)
				}
			} else {
				n, err := c.doTee(uintptr(c.buf[0]), wrc, total)
				if err != nil || (total > 0 && n < total) {
					if err != unix.EAGAIN && total > 0 && n < total {
						c.mu.Lock()
						c._lastErr = err
						c.mu.Unlock()
						evict = append(evict, i)
						continue
					}
				}
				if total == 0 {
					total = n
				}
			}
		}

		// We only splice on the last writer
		// If for some reason we couldn't do that then we need to drain that data
		// from the buffer.
		if !spliced && total > 0 {
			buf := make([]byte, 32*1024)
			nn := total
			for nn > 0 {
				if int64(len(buf)) > nn {
					buf = buf[:nn]
				}
				n, err := unix.Read(c.buf[0], buf)
				if n > 0 {
					nn -= int64(n)
				}
				if err != nil {
					if err != unix.EAGAIN {
						c.setClosedErr(err)
					}
					return true
				}
			}
		}

		return true
	})

	for n, i := range evict {
		c.writers = append(c.writers[:i-n], c.writers[i-n+1:]...)
	}

	if err != nil {
		c.setClosedErr(err)
	}
}

// Copier calls doSplice when it is copying to the last (or only) writer.
//
// When `total` is 0, this should be the *only* writer.
// In such a case we only want to splice until EAGAIN (or some fatal error).
//
// When `total` is greater than zero we need to keep trying until either
// we have written `total` bytes OR some fatal error (*not* EGAIN).
func (c *Copier) doSplice(rfd uintptr, wrc syscall.RawConn, total int64) (int64, error) {
	var (
		written   int64
		spliceErr error
	)

	writeErr := wrc.Write(func(wfd uintptr) bool {
		n, err := splice(int(rfd), int(wfd), total-written)
		if n > 0 {
			written += n
		}
		spliceErr = err

		if n == 0 && spliceErr == nil {
			spliceErr = io.EOF
		}

		return true
	})
	if writeErr != nil {
		return written, writeErr
	}
	return written, spliceErr
}

func (c *Copier) doTee(rfd uintptr, wrc syscall.RawConn, total int64) (int64, error) {
	var (
		written int64
		teeErr  error
	)

	writeErr := wrc.Write(func(wfd uintptr) bool {
		n, err := tee(int(rfd), int(wfd), total-written)
		if n > 0 {
			written += n
		}
		teeErr = err

		if n == 0 {
			if err == nil {
				teeErr = io.EOF
			}
		}

		return true
	})

	if writeErr != nil {
		return written, writeErr
	}
	return written, teeErr
}
