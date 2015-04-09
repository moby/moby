package daemon

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/utils"
)

func (c *Container) AttachWithLogs(stdin io.ReadCloser, stdout, stderr io.Writer, logs, stream bool) error {
	if logs {
		cLog, err := c.ReadLog("json")
		if err != nil && os.IsNotExist(err) {
			// Legacy logs
			logrus.Debugf("Old logs format")
			if stdout != nil {
				cLog, err := c.ReadLog("stdout")
				if err != nil {
					logrus.Errorf("Error reading logs (stdout): %s", err)
				} else if _, err := io.Copy(stdout, cLog); err != nil {
					logrus.Errorf("Error streaming logs (stdout): %s", err)
				}
			}
			if stderr != nil {
				cLog, err := c.ReadLog("stderr")
				if err != nil {
					logrus.Errorf("Error reading logs (stderr): %s", err)
				} else if _, err := io.Copy(stderr, cLog); err != nil {
					logrus.Errorf("Error streaming logs (stderr): %s", err)
				}
			}
		} else if err != nil {
			logrus.Errorf("Error reading logs (json): %s", err)
		} else {
			dec := json.NewDecoder(cLog)
			for {
				l := &jsonlog.JSONLog{}

				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					logrus.Errorf("Error streaming logs: %s", err)
					break
				}
				if l.Stream == "stdout" && stdout != nil {
					io.WriteString(stdout, l.Log)
				}
				if l.Stream == "stderr" && stderr != nil {
					io.WriteString(stderr, l.Log)
				}
			}
		}
	}

	//stream
	if stream {
		var stdinPipe io.ReadCloser
		if stdin != nil {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				defer logrus.Debugf("Closing buffered stdin pipe")
				io.Copy(w, stdin)
			}()
			stdinPipe = r
		}
		<-c.Attach(stdinPipe, stdout, stderr)
		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if c.Config.StdinOnce && !c.Config.Tty {
			c.WaitStop(-1 * time.Second)
		}
	}
	return nil
}

func (c *Container) Attach(stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) chan error {
	return attach(&c.StreamConfig, c.Config.OpenStdin, c.Config.StdinOnce, c.Config.Tty, stdin, stdout, stderr)
}

func attach(streamConfig *StreamConfig, openStdin, stdinOnce, tty bool, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) chan error {
	var (
		cStdout, cStderr io.ReadCloser
		cStdin           io.WriteCloser
		wg               sync.WaitGroup
		errors           = make(chan error, 3)
	)

	if stdin != nil && openStdin {
		cStdin = streamConfig.StdinPipe()
		wg.Add(1)
	}

	if stdout != nil {
		cStdout = streamConfig.StdoutPipe()
		wg.Add(1)
	}

	if stderr != nil {
		cStderr = streamConfig.StderrPipe()
		wg.Add(1)
	}

	// Connect stdin of container to the http conn.
	go func() {
		if stdin == nil || !openStdin {
			return
		}
		logrus.Debugf("attach: stdin: begin")
		defer func() {
			if stdinOnce && !tty {
				cStdin.Close()
			} else {
				// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
				if cStdout != nil {
					cStdout.Close()
				}
				if cStderr != nil {
					cStderr.Close()
				}
			}
			wg.Done()
			logrus.Debugf("attach: stdin: end")
		}()

		var err error
		if tty {
			_, err = utils.CopyEscapable(cStdin, stdin)
		} else {
			_, err = io.Copy(cStdin, stdin)

		}
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			logrus.Errorf("attach: stdin: %s", err)
			errors <- err
			return
		}
	}()

	attachStream := func(name string, stream io.Writer, streamPipe io.ReadCloser) {
		if stream == nil {
			return
		}
		defer func() {
			// Make sure stdin gets closed
			if stdin != nil {
				stdin.Close()
			}
			streamPipe.Close()
			wg.Done()
			logrus.Debugf("attach: %s: end", name)
		}()

		logrus.Debugf("attach: %s: begin", name)
		_, err := io.Copy(stream, streamPipe)
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			logrus.Errorf("attach: %s: %v", name, err)
			errors <- err
		}
	}

	go attachStream("stdout", stdout, cStdout)
	go attachStream("stderr", stderr, cStderr)

	return promise.Go(func() error {
		wg.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				return err
			}
		}
		return nil
	})
}
