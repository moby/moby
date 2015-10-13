// +build windows

package windows

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
)

// General comment. Handling I/O for a container is very different to Linux.
// We use a named pipe to HCS to copy I/O both in and out of the container,
// very similar to how docker daemon communicates with a CLI.

// startStdinCopy asynchronously copies an io.Reader to the container's
// process's stdin pipe and closes the pipe when there is no more data to copy.
func startStdinCopy(dst io.WriteCloser, src io.Reader) {

	// Anything that comes from the client stdin should be copied
	// across to the stdin named pipe of the container.
	go func() {
		defer dst.Close()
		bytes, err := io.Copy(dst, src)
		log := fmt.Sprintf("Copied %d bytes from stdin.", bytes)
		if err != nil {
			log = log + " err=" + err.Error()
		}
		logrus.Debugf(log)
	}()
}

// startStdouterrCopy asynchronously copies data from the container's process's
// stdout or stderr pipe to an io.Writer and closes the pipe when there is no
// more data to copy.
func startStdouterrCopy(dst io.Writer, src io.ReadCloser, name string) {
	// Anything that comes from the container named pipe stdout/err should be copied
	// across to the stdout/err of the client
	go func() {
		defer src.Close()
		bytes, err := io.Copy(dst, src)
		log := fmt.Sprintf("Copied %d bytes from %s.", bytes, name)
		if err != nil {
			log = log + " err=" + err.Error()
		}
		logrus.Debugf(log)
	}()
}

// setupPipes starts the asynchronous copying of data to and from the named
// pipes used byt he HCS for the std handles.
func setupPipes(stdin io.WriteCloser, stdout, stderr io.ReadCloser, pipes *execdriver.Pipes) {
	if stdin != nil {
		startStdinCopy(stdin, pipes.Stdin)
	}
	if stdout != nil {
		startStdouterrCopy(pipes.Stdout, stdout, "stdout")
	}
	if stderr != nil {
		startStdouterrCopy(pipes.Stderr, stderr, "stderr")
	}
}
