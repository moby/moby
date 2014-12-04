package daemon

import (
	"encoding/json"
	"io"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/utils"
)

func (daemon *Daemon) ContainerAttach(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("Usage: %s CONTAINER\n", job.Name)
	}

	var (
		name   = job.Args[0]
		logs   = job.GetenvBool("logs")
		stream = job.GetenvBool("stream")
		stdin  = job.GetenvBool("stdin")
		stdout = job.GetenvBool("stdout")
		stderr = job.GetenvBool("stderr")
	)

	container := daemon.Get(name)
	if container == nil {
		return job.Errorf("No such container: %s", name)
	}

	//logs
	if logs {
		cLog, err := container.ReadLog("json")
		if err != nil && os.IsNotExist(err) {
			// Legacy logs
			log.Debugf("Old logs format")
			if stdout {
				cLog, err := container.ReadLog("stdout")
				if err != nil {
					log.Errorf("Error reading logs (stdout): %s", err)
				} else if _, err := io.Copy(job.Stdout, cLog); err != nil {
					log.Errorf("Error streaming logs (stdout): %s", err)
				}
			}
			if stderr {
				cLog, err := container.ReadLog("stderr")
				if err != nil {
					log.Errorf("Error reading logs (stderr): %s", err)
				} else if _, err := io.Copy(job.Stderr, cLog); err != nil {
					log.Errorf("Error streaming logs (stderr): %s", err)
				}
			}
		} else if err != nil {
			log.Errorf("Error reading logs (json): %s", err)
		} else {
			dec := json.NewDecoder(cLog)
			for {
				l := &jsonlog.JSONLog{}

				if err := dec.Decode(l); err == io.EOF {
					break
				} else if err != nil {
					log.Errorf("Error streaming logs: %s", err)
					break
				}
				if l.Stream == "stdout" && stdout {
					io.WriteString(job.Stdout, l.Log)
				}
				if l.Stream == "stderr" && stderr {
					io.WriteString(job.Stderr, l.Log)
				}
			}
		}
	}

	//stream
	if stream {
		var (
			cStdin           io.ReadCloser
			cStdout, cStderr io.Writer
		)

		if stdin {
			r, w := io.Pipe()
			go func() {
				defer w.Close()
				defer log.Debugf("Closing buffered stdin pipe")
				io.Copy(w, job.Stdin)
			}()
			cStdin = r
		}
		if stdout {
			cStdout = job.Stdout
		}
		if stderr {
			cStderr = job.Stderr
		}

		<-daemon.attach(&container.StreamConfig, container.Config.OpenStdin, container.Config.StdinOnce, container.Config.Tty, cStdin, cStdout, cStderr)
		// If we are in stdinonce mode, wait for the process to end
		// otherwise, simply return
		if container.Config.StdinOnce && !container.Config.Tty {
			container.WaitStop(-1 * time.Second)
		}
	}
	return engine.StatusOK
}

func (daemon *Daemon) attach(streamConfig *StreamConfig, openStdin, stdinOnce, tty bool, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) chan error {
	var (
		cStdout, cStderr io.ReadCloser
		cStdin           io.WriteCloser
		nJobs            int
	)

	if stdin != nil && openStdin {
		cStdin = streamConfig.StdinPipe()
		nJobs++
	}

	if stdout != nil {
		cStdout = streamConfig.StdoutPipe()
		nJobs++
	}

	if stderr != nil {
		cStderr = streamConfig.StderrPipe()
		nJobs++
	}

	errors := make(chan error, nJobs)

	// Connect stdin of container to the http conn.
	if stdin != nil && openStdin {
		// Get the stdin pipe.
		cStdin = streamConfig.StdinPipe()
		go func() {
			log.Debugf("attach: stdin: begin")
			defer func() {
				if stdinOnce && !tty {
					defer cStdin.Close()
				} else {
					// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
					if cStdout != nil {
						cStdout.Close()
					}
					if cStderr != nil {
						cStderr.Close()
					}
				}
				log.Debugf("attach: stdin: end")
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
				log.Errorf("attach: stdin: %s", err)
			}
			errors <- err
		}()
	}

	attachStream := func(name string, stream io.Writer, streamPipe io.ReadCloser) {
		if stream == nil {
			return
		}
		defer func() {
			// Make sure stdin gets closed
			if stdinOnce && cStdin != nil {
				stdin.Close()
				cStdin.Close()
			}
			streamPipe.Close()
		}()

		log.Debugf("attach: %s: begin", name)
		defer log.Debugf("attach: %s: end", name)
		_, err := io.Copy(stream, streamPipe)
		if err == io.ErrClosedPipe {
			err = nil
		}
		if err != nil {
			log.Errorf("attach: %s: %v", name, err)
		}
		errors <- err
	}

	go attachStream("stdout", stdout, cStdout)
	go attachStream("stderr", stderr, cStderr)

	return promise.Go(func() error {
		for i := 0; i < nJobs; i++ {
			log.Debugf("attach: waiting for job %d/%d", i+1, nJobs)
			err := <-errors
			if err != nil {
				log.Errorf("attach: job %d returned error %s, aborting all jobs", i+1, err)
				return err
			}
			log.Debugf("attach: job %d completed successfully", i+1)
		}
		log.Debugf("attach: all jobs completed successfully")
		return nil
	})
}
