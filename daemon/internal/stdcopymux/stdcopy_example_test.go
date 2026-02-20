package stdcopymux_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/moby/moby/client/pkg/stdcopy"
	"github.com/moby/moby/v2/daemon/internal/stdcopymux"
)

func ExampleNewStdWriter() {
	muxReader, muxStream := io.Pipe()
	defer func() { _ = muxStream.Close() }()

	// Start demuxing before the daemon starts writing.
	done := make(chan error, 1)
	go func() {
		// using os.Stdout for both, otherwise output doesn't show up in the example.
		osStdout := os.Stdout
		osStderr := os.Stdout
		_, err := stdcopy.StdCopy(osStdout, osStderr, muxReader)
		done <- err
	}()

	// daemon writing to stdout, stderr, and systemErr.
	stdout := stdcopymux.NewStdWriter(muxStream, stdcopy.Stdout)
	stderr := stdcopymux.NewStdWriter(muxStream, stdcopy.Stderr)
	systemErr := stdcopymux.NewStdWriter(muxStream, stdcopy.Systemerr)

	for range 10 {
		_, _ = fmt.Fprintln(stdout, "hello from stdout")
		_, _ = fmt.Fprintln(stderr, "hello from stderr")
		time.Sleep(50 * time.Millisecond)
	}
	_, _ = fmt.Fprintln(systemErr, errors.New("something went wrong"))

	// Wait for the demuxer to finish.
	if err := <-done; err != nil {
		fmt.Println(err)
	}

	// Output:
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// hello from stdout
	// hello from stderr
	// error from daemon in stream: something went wrong
}
