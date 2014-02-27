package beam

import (
	"io"
	"strings"
)

type Job struct {
	Msg    Message
	Name   string
	Args   []string
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func JobHandler(f func(*Job)) Stream {
	inside, outside := Pipe()
	go func() {
		defer inside.Close()
		for {
			msg, err := inside.Receive()
			if err != nil {
				return
			}
			if msg.Stream == nil {
				msg.Stream = DevNull
			}
			parts := strings.Split(string(msg.Data), " ")
			// Setup default stdout
			// FIXME: The job handler can change it before calling job.Send()
			// For example if it wants to send a file (eg. 'exec')
			func() error {
				stdout, stdoutStream := PipeWriter()
				if err := msg.Stream.Send(Message{Data: []byte("stdout"), Stream: stdoutStream}); err != nil {
					return err
				}
				stderr, stderrStream := PipeWriter()
				if err := msg.Stream.Send(Message{Data: []byte("stderr"), Stream: stderrStream}); err != nil {
					return err
				}
				job := &Job{
					Msg:    msg,
					Name:   parts[0],
					Stdout: stdout,
					Stderr: stderr,
				}
				if len(parts) > 1 {
					job.Args = parts[1:]
				}
				f(job)
				return nil
			}()
		}
	}()
	return outside
}

func PipeWriter() (io.WriteCloser, Stream) {
	r, w := io.Pipe()
	inside, outside := Pipe()
	go func() {
		defer inside.Close()
		defer r.Close()
		for {
			data := make([]byte, 4098)
			n, err := r.Read(data)
			if n > 0 {
				if inside.Send(Message{Data: data[:n]}); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return w, outside
}
