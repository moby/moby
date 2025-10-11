package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"sync"
)

// Generic stream implementation.
type stream[T any] struct {
	rc    io.ReadCloser
	close func() error
}

type Stream[T any] interface {
	io.ReadCloser
	Messages(ctx context.Context) iter.Seq2[T, error]
}

// NewMessageStream constructs a typed stream that yields values of T.
func NewMessageStream[T any](rc io.ReadCloser) Stream[T] {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return &stream[T]{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

// Read implements io.Reader.
func (r *stream[T]) Read(p []byte) (int, error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.Closer.
func (r *stream[T]) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

// Messages decodes the response stream as a sequence of T.
// If the stream ends or the context is canceled, the underlying reader is closed.
func (r *stream[T]) Messages(ctx context.Context) iter.Seq2[T, error] {
	context.AfterFunc(ctx, func() { _ = r.Close() })
	dec := json.NewDecoder(r)

	return func(yield func(T, error) bool) {
		defer func() { _ = r.Close() }()
		for {
			var msg T
			err := dec.Decode(&msg)
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				_ = yield(msg, ctx.Err())
				return
			}
			if !yield(msg, err) {
				return
			}
		}
	}
}

// Wait waits for operation to complete and detects errors reported as JSONMessage
func (r *stream[T]) Wait(ctx context.Context) error {
	for _, err := range r.Messages(ctx) {
		if err != nil {
			return err
		}
	}
	return nil
}
