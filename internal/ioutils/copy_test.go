package ioutils

import (
	"bytes"
	"context"
	"testing"
	"time"
)

type blockingReader struct{}

func (r blockingReader) Read(p []byte) (int, error) {
	time.Sleep(time.Second)
	return 0, nil
}

func TestCopyCtx(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*5)
	defer cancel()

	dst := new(bytes.Buffer)

	finished := make(chan struct{})

	go func() {
		CopyCtx(ctx, dst, blockingReader{})
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(time.Millisecond * 100):
		t.Fatal("CopyCtx did not return after context was cancelled")
	}
}
