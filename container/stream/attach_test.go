package stream

import (
	"context"
	"io"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// Make sure when there is no I/O on a stream that the goroutines do not get blocked after the container exits.
func TestAttachNoIO(t *testing.T) {
	t.Run("stdin only", func(t *testing.T) {
		stdinR, _ := io.Pipe()
		defer stdinR.Close()
		testStreamCopy(t, stdinR, nil, nil)
	})

	t.Run("stdout only", func(t *testing.T) {
		_, w := io.Pipe()
		defer w.Close()
		testStreamCopy(t, nil, w, nil)
	})

	t.Run("stderr only", func(t *testing.T) {
		_, w := io.Pipe()
		defer w.Close()
		testStreamCopy(t, nil, nil, w)
	})

	t.Run("stdout+stderr", func(t *testing.T) {
		_, stdoutW := io.Pipe()
		defer stdoutW.Close()
		_, stderrW := io.Pipe()
		defer stderrW.Close()

		testStreamCopy(t, nil, stdoutW, stderrW)
	})

	t.Run("stdin+stdout", func(t *testing.T) {
		stdin, _ := io.Pipe()
		defer stdin.Close()
		_, stdout := io.Pipe()
		defer stdout.Close()

		testStreamCopy(t, stdin, stdout, nil)
	})

	t.Run("stdin+stderr", func(t *testing.T) {
		stdin, _ := io.Pipe()
		defer stdin.Close()
		_, stderr := io.Pipe()
		defer stderr.Close()

		testStreamCopy(t, stdin, nil, stderr)
	})

	t.Run("stdin+stdout+stderr", func(t *testing.T) {
		stdinR, _ := io.Pipe()
		defer stdinR.Close()
		stdoutR, stdoutW := io.Pipe()
		defer stdoutR.Close()
		stderrR, stderrW := io.Pipe()
		defer stderrR.Close()
		testStreamCopy(t, stdinR, stdoutW, stderrW)
	})
}

func testStreamCopy(t *testing.T, stdin io.ReadCloser, stdout, stderr io.WriteCloser) {
	cfg := AttachConfig{
		UseStdin:  stdin != nil,
		UseStdout: stdout != nil,
		UseStderr: stderr != nil,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
	}

	sc := NewConfig()
	sc.AttachStreams(&cfg)
	defer sc.CloseStreams()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chErr := sc.CopyStreams(ctx, &cfg)

	select {
	case err := <-chErr:
		assert.NilError(t, err)
	default:
	}

	cancel()

	select {
	case err := <-chErr:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for CopyStreams to exit")
	}
}
