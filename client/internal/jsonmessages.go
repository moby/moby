package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"sync"
)

func NewJSONMessageStream[T any](rc io.ReadCloser) stream[T] {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return stream[T]{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

type stream[T any] struct {
	rc    io.ReadCloser
	close func() error
}

// Read implements io.ReadCloser
func (r stream[T]) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.ReadCloser
func (r stream[T]) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

// JSONMessages decodes the response stream as a sequence of JSONMessages.
// if stream ends or context is cancelled, the underlying [io.Reader] is closed.
func (r stream[T]) JSONMessages(ctx context.Context) iter.Seq2[T, error] {
	context.AfterFunc(ctx, func() {
		_ = r.Close()
	})
	dec := json.NewDecoder(r)
	return func(yield func(T, error) bool) {
		defer r.Close()
		for {
			var jm T
			err := dec.Decode(&jm)
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				yield(jm, ctx.Err())
				return
			}
			if !yield(jm, err) {
				return
			}
		}
	}
}

// Wait waits for operation to complete and detects errors reported as JSONMessage
func (r stream[T]) Wait(ctx context.Context) error {
	for _, err := range r.JSONMessages(ctx) {
		if err != nil {
			return err
		}
	}
	return nil
}
