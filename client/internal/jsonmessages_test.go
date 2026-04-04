package internal_test

import (
	"context"
	"io"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/moby/moby/client/internal"
)

func TestStreamWait(t *testing.T) {
	tests := []struct {
		doc      string
		input    string
		expError string
		expType  func(error) bool
	}{
		{
			doc:   "success",
			input: `{"status":"ok"}`,
		},
		{
			doc:      "internal server error",
			input:    `{"errorDetail": {"code": 500, "message": "something went wrong"}}`,
			expError: "something went wrong",
			expType:  cerrdefs.IsInternal,
		},
		{
			doc:      "access error",
			input:    `{"errorDetail": {"code": 403, "message": "access denied"}}`,
			expError: "access denied",
			expType:  cerrdefs.IsPermissionDenied,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			r := io.NopCloser(strings.NewReader(tc.input))
			s := internal.NewJSONMessageStream(r)

			err := s.Wait(t.Context())
			if tc.expError == "" {
				assert.NilError(t, err)
				return
			}
			assert.Check(t, is.ErrorContains(err, tc.expError))
			assert.Check(t, is.ErrorType(err, tc.expType))
		})
	}
}

func TestStreamWait_ContextCanceled(t *testing.T) {
	rc := newBlockingReadCloser()
	s := internal.NewJSONMessageStream(rc)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Wait(ctx)
	}()
	cancel()

	err := <-done
	assert.ErrorIs(t, err, context.Canceled)
}

type blockingReadCloser struct {
	done chan struct{}
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{done: make(chan struct{})}
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	<-r.done
	return 0, io.ErrClosedPipe
}

func (r *blockingReadCloser) Close() error {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
	return nil
}
