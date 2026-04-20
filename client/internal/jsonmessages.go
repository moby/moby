package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"sync"

	"github.com/containerd/errdefs/pkg/errhttp"

	"github.com/moby/moby/api/types/jsonstream"
)

func NewJSONMessageStream(rc io.ReadCloser) Stream {
	if rc == nil {
		panic("nil io.ReadCloser")
	}
	return Stream{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
}

type Stream struct {
	rc    io.ReadCloser
	close func() error
}

// Read implements io.ReadCloser
func (r Stream) Read(p []byte) (n int, err error) {
	if r.rc == nil {
		return 0, io.EOF
	}
	return r.rc.Read(p)
}

// Close implements io.ReadCloser
func (r Stream) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}

var _ io.ReadCloser = Stream{}

// JSONMessages decodes the response stream as a sequence of [jsonstream.Message].
// The underlying [io.Reader] is closed when the stream ends or if the context
// is cancelled.
func (r Stream) JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error] {
	stop := context.AfterFunc(ctx, func() {
		_ = r.Close()
	})
	return func(yield func(jsonstream.Message, error) bool) {
		defer func() {
			stop() // unregister AfterFunc
			_ = r.Close()
		}()

		dec := json.NewDecoder(r)
		for {
			var jm jsonstream.Message
			if err := dec.Decode(&jm); err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				if err := ctx.Err(); err != nil {
					// Do not return decoding errors if the context was
					// cancelled, because the decoding errors may be due
					// to the context being cancelled.
					yield(jsonstream.Message{}, err)
					return
				}
				yield(jsonstream.Message{}, err)
				return
			}
			if !yield(jm, nil) {
				return
			}
		}
	}
}

// Wait consumes the stream until completion.
//
// It returns nil if the operation completes successfully. Errors are
// returned if the context is canceled, a decoding/transport failure
// occurs, or a JSON message reports an error ([jsonstream.Message.Error]).
func (r Stream) Wait(ctx context.Context) error {
	for jm, err := range r.JSONMessages(ctx) {
		if err != nil {
			// decode, transport and context cancellation errors.
			return err
		}
		if jm.Error != nil {
			// push/pull failures.
			return httpErrorFromStatusCode(jm.Error, jm.Error.Code)
		}
	}
	return nil
}

type httpError struct {
	err    error
	errdef error
}

func (e *httpError) Error() string {
	return e.err.Error()
}

func (e *httpError) Unwrap() error {
	return e.err
}

func (e *httpError) Is(target error) bool {
	return errors.Is(e.errdef, target)
}

// httpErrorFromStatusCode creates an errdef error, based on the provided HTTP status-code
//
// TODO(thaJeztah): unify with the implementation in client and move to an internal package
// see https://github.com/moby/moby/blob/client/v0.4.0/client/errors.go#L76-L114
func httpErrorFromStatusCode(err error, statusCode int) error {
	if err == nil {
		return nil
	}

	return &httpError{
		err:    err,
		errdef: errhttp.ToNative(statusCode),
	}
}
