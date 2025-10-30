package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"sync"

	"github.com/moby/moby/api/types/jsonstream"
)

func NewJSONMessageStream(rc io.ReadCloser) stream {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return stream{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

type stream struct {
	rc    io.ReadCloser
	close func() error
}

// Read implements io.ReadCloser
func (r stream) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.ReadCloser
func (r stream) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

// JSONMessages decodes the response stream as a sequence of JSONMessages.
// if stream ends or context is cancelled, the underlying [io.Reader] is closed.
func (r stream) JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error] {
	context.AfterFunc(ctx, func() {
		_ = r.Close()
	})
	dec := json.NewDecoder(r)
	return func(yield func(jsonstream.Message, error) bool) {
		defer r.Close()
		for {
			var jm jsonstream.Message
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
func (r stream) Wait(ctx context.Context) error {
	for _, err := range r.JSONMessages(ctx) {
		if err != nil {
			return err
		}
	}
	return nil
}
