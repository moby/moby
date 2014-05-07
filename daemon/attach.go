package daemon

import (
	"io"

	"github.com/dotcloud/docker/utils"
)

func (daemon *Daemon) Attach(container *Container, stdin io.ReadCloser, stdinCloser io.Closer, stdout io.Writer, stderr io.Writer) chan error {
	var (
		cStdout, cStderr io.ReadCloser
		nJobs            int
		errors           = make(chan error, 3)
	)

	if stdin != nil && container.Config.OpenStdin {
		nJobs += 1
		if cStdin, err := container.StdinPipe(); err != nil {
			errors <- err
		} else {
			go func() {
				utils.Debugf("attach: stdin: begin")
				defer utils.Debugf("attach: stdin: end")
				// No matter what, when stdin is closed (io.Copy unblock), close stdout and stderr
				if container.Config.StdinOnce && !container.Config.Tty {
					defer cStdin.Close()
				} else {
					defer func() {
						if cStdout != nil {
							cStdout.Close()
						}
						if cStderr != nil {
							cStderr.Close()
						}
					}()
				}
				if container.Config.Tty {
					_, err = utils.CopyEscapable(cStdin, stdin)
				} else {
					_, err = io.Copy(cStdin, stdin)
				}
				if err == io.ErrClosedPipe {
					err = nil
				}
				if err != nil {
					utils.Errorf("attach: stdin: %s", err)
				}
				errors <- err
			}()
		}
	}
	if stdout != nil {
		nJobs += 1
		if p, err := container.StdoutPipe(); err != nil {
			errors <- err
		} else {
			cStdout = p
			go func() {
				utils.Debugf("attach: stdout: begin")
				defer utils.Debugf("attach: stdout: end")
				// If we are in StdinOnce mode, then close stdin
				if container.Config.StdinOnce && stdin != nil {
					defer stdin.Close()
				}
				if stdinCloser != nil {
					defer stdinCloser.Close()
				}
				_, err := io.Copy(stdout, cStdout)
				if err == io.ErrClosedPipe {
					err = nil
				}
				if err != nil {
					utils.Errorf("attach: stdout: %s", err)
				}
				errors <- err
			}()
		}
	} else {
		go func() {
			if stdinCloser != nil {
				defer stdinCloser.Close()
			}
			if cStdout, err := container.StdoutPipe(); err != nil {
				utils.Errorf("attach: stdout pipe: %s", err)
			} else {
				io.Copy(&utils.NopWriter{}, cStdout)
			}
		}()
	}
	if stderr != nil {
		nJobs += 1
		if p, err := container.StderrPipe(); err != nil {
			errors <- err
		} else {
			cStderr = p
			go func() {
				utils.Debugf("attach: stderr: begin")
				defer utils.Debugf("attach: stderr: end")
				// If we are in StdinOnce mode, then close stdin
				if container.Config.StdinOnce && stdin != nil {
					defer stdin.Close()
				}
				if stdinCloser != nil {
					defer stdinCloser.Close()
				}
				_, err := io.Copy(stderr, cStderr)
				if err == io.ErrClosedPipe {
					err = nil
				}
				if err != nil {
					utils.Errorf("attach: stderr: %s", err)
				}
				errors <- err
			}()
		}
	} else {
		go func() {
			if stdinCloser != nil {
				defer stdinCloser.Close()
			}

			if cStderr, err := container.StderrPipe(); err != nil {
				utils.Errorf("attach: stdout pipe: %s", err)
			} else {
				io.Copy(&utils.NopWriter{}, cStderr)
			}
		}()
	}

	return utils.Go(func() error {
		defer func() {
			if cStdout != nil {
				cStdout.Close()
			}
			if cStderr != nil {
				cStderr.Close()
			}
		}()

		// FIXME: how to clean up the stdin goroutine without the unwanted side effect
		// of closing the passed stdin? Add an intermediary io.Pipe?
		for i := 0; i < nJobs; i += 1 {
			utils.Debugf("attach: waiting for job %d/%d", i+1, nJobs)
			if err := <-errors; err != nil {
				utils.Errorf("attach: job %d returned error %s, aborting all jobs", i+1, err)
				return err
			}
			utils.Debugf("attach: job %d completed successfully", i+1)
		}
		utils.Debugf("attach: all jobs completed successfully")
		return nil
	})
}
