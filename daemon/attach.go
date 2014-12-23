package daemon

import (
	"encoding/json"
	"io"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/ioutils"
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
		nJobs            int
		errors           = make(chan error, 3)
	)

	// Connect stdin of container to the http conn.
	if stdin != nil && openStdin {
		nJobs++
		// Get the stdin pipe.
		if cStdin, err := streamConfig.StdinPipe(); err != nil {
			errors <- err
		} else {
			go func() {
				log.Debugf("attach: stdin: begin")
				defer log.Debugf("attach: stdin: end")
				if stdinOnce && !tty {
					defer cStdin.Close()
				} else {
					// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
					defer func() {
						if cStdout != nil {
							cStdout.Close()
						}
						if cStderr != nil {
							cStderr.Close()
						}
					}()
				}
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
	}
	if stdout != nil {
		nJobs++
		// Get a reader end of a pipe that is attached as stdout to the container.
		if p, err := streamConfig.StdoutPipe(); err != nil {
			errors <- err
		} else {
			cStdout = p
			go func() {
				log.Debugf("attach: stdout: begin")
				defer log.Debugf("attach: stdout: end")
				// If we are in StdinOnce mode, then close stdin
				if stdinOnce && stdin != nil {
					defer stdin.Close()
				}
				_, err := io.Copy(stdout, cStdout)
				if err == io.ErrClosedPipe {
					err = nil
				}
				if err != nil {
					log.Errorf("attach: stdout: %s", err)
				}
				errors <- err
			}()
		}
	} else {
		// Point stdout of container to a no-op writer.
		go func() {
			if cStdout, err := streamConfig.StdoutPipe(); err != nil {
				log.Errorf("attach: stdout pipe: %s", err)
			} else {
				io.Copy(&ioutils.NopWriter{}, cStdout)
			}
		}()
	}
	if stderr != nil {
		nJobs++
		if p, err := streamConfig.StderrPipe(); err != nil {
			errors <- err
		} else {
			cStderr = p
			go func() {
				log.Debugf("attach: stderr: begin")
				defer log.Debugf("attach: stderr: end")
				// If we are in StdinOnce mode, then close stdin
				// Why are we closing stdin here and above while handling stdout?
				if stdinOnce && stdin != nil {
					defer stdin.Close()
				}
				_, err := io.Copy(stderr, cStderr)
				if err == io.ErrClosedPipe {
					err = nil
				}
				if err != nil {
					log.Errorf("attach: stderr: %s", err)
				}
				errors <- err
			}()
		}
	} else {
		// Point stderr at a no-op writer.
		go func() {
			if cStderr, err := streamConfig.StderrPipe(); err != nil {
				log.Errorf("attach: stdout pipe: %s", err)
			} else {
				io.Copy(&ioutils.NopWriter{}, cStderr)
			}
		}()
	}

	return promise.Go(func() error {
		defer func() {
			if cStdout != nil {
				cStdout.Close()
			}
			if cStderr != nil {
				cStderr.Close()
			}
		}()

		for i := 0; i < nJobs; i++ {
			log.Debugf("attach: waiting for job %d/%d", i+1, nJobs)
			if err := <-errors; err != nil {
				log.Errorf("attach: job %d returned error %s, aborting all jobs", i+1, err)
				return err
			}
			log.Debugf("attach: job %d completed successfully", i+1)
		}
		log.Debugf("attach: all jobs completed successfully")
		return nil
	})
}
