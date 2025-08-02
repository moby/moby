package stdcopy_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
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
	stdout := stdcopy.NewStdWriter(muxStream, stdcopy.Stdout)
	stderr := stdcopy.NewStdWriter(muxStream, stdcopy.Stderr)
	systemErr := stdcopy.NewStdWriter(muxStream, stdcopy.Systemerr)

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
