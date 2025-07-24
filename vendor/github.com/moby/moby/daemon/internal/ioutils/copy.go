package ioutils

import (
	"context"
	"io"
)

// CopyCtx copies from src to dst until either EOF is reached on src or a context is cancelled.
// The writer is not closed when the context is cancelled.
//
// After CopyCtx exits due to context cancellation, the goroutine that performed
// the copy may still be running if either the reader or writer blocks.
func CopyCtx(ctx context.Context, dst io.Writer, src io.Reader) (n int64, err error) {
	copyDone := make(chan struct{})

	src = &readerCtx{ctx: ctx, r: src}

	go func() {
		n, err = io.Copy(dst, src)
		close(copyDone)
	}()

	select {
	case <-ctx.Done():
		return -1, ctx.Err()
	case <-copyDone:
	}

	return n, err
}

type readerCtx struct {
	ctx context.Context
	r   io.Reader
}

// NewCtxReader wraps the given reader with a reader that doesn't proceed with
// reading if the context is done.
//
// Note: Read will still block if the underlying reader blocks.
func NewCtxReader(ctx context.Context, r io.Reader) io.Reader {
	return &readerCtx{ctx: ctx, r: r}
}

func (r *readerCtx) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	n, outErr := r.r.Read(p)

	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return n, outErr
}
