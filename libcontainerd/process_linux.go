package libcontainerd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	containerd "github.com/docker/containerd/api/grpc/types"
	"golang.org/x/net/context"
)

var fdNames = map[int]string{
	syscall.Stdin:  "stdin",
	syscall.Stdout: "stdout",
	syscall.Stderr: "stderr",
}

// process keeps the state for both main container process and exec process.
type process struct {
	processCommon

	// Platform specific fields are below here.
	dir string
}

func (p *process) openFifos(terminal bool) (*IOPipe, error) {
	bundleDir := p.dir
	if err := os.MkdirAll(bundleDir, 0700); err != nil {
		return nil, err
	}

	for i := 0; i < 3; i++ {
		f := p.fifo(i)
		if err := syscall.Mkfifo(f, 0700); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("mkfifo: %s %v", f, err)
		}
	}

	io := &IOPipe{}

	io.Stdout = openReaderFromFifo(p.fifo(syscall.Stdout))
	if !terminal {
		io.Stderr = openReaderFromFifo(p.fifo(syscall.Stderr))
	} else {
		io.Stderr = emptyReader{}
	}

	io.Stdin = &stdinWriter{
		path: p.fifo(syscall.Stdin),
		cb: func() error {
			_, err := p.client.remote.apiClient.UpdateProcess(context.Background(), &containerd.UpdateProcessRequest{
				Id:         p.containerID,
				Pid:        p.friendlyName,
				CloseStdin: true,
			})
			return err
		},
	}

	return io, nil
}

func (p *process) closeFifos(io *IOPipe) {
	io.Stdin.Close()
	closeReaderFifo(p.fifo(syscall.Stdout))
	closeReaderFifo(p.fifo(syscall.Stderr))
}

type stdinWriter struct {
	sync.Mutex
	path   string
	cb     func() error
	f      *os.File
	closed bool
	ready  chan struct{}
}

func (s *stdinWriter) Write(b []byte) (int, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return 0, fmt.Errorf("writing on closed stdin")
	}

	if s.f == nil {
		s.ready = make(chan struct{})
		go func() {
			var err error
			s.f, err = os.OpenFile(s.path, syscall.O_WRONLY, 0)
			if err == nil {
				close(s.ready)
			}
		}()
		select {
		case <-s.ready:
		case <-time.After(500 * time.Millisecond):
			// this case is for apps closing stdin on startup before we attach and for restores that already have closed stdin.
			closeWriterFifo(s.path)
			return 0, fmt.Errorf("could not open %v for writing", s.path)
		}
	}
	return s.f.Write(b)
}

func (s *stdinWriter) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.f != nil {
		s.f.Close()
	}
	s.closed = true
	return s.cb()
}

type emptyReader struct{}

func (r emptyReader) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func openReaderFromFifo(fn string) io.Reader {
	r, w := io.Pipe()
	c := make(chan struct{})
	go func() {
		close(c)
		stdoutf, err := os.OpenFile(fn, syscall.O_RDONLY, 0)
		if err != nil {
			r.CloseWithError(err)
		}
		if _, err := io.Copy(w, stdoutf); err != nil {
			r.CloseWithError(err)
		}
		w.Close()
		stdoutf.Close()
	}()
	<-c // wait for the goroutine to get scheduled and syscall to block
	return r
}

// closeReaderFifo closes fifo that may be blocked on open by opening the write side.
func closeReaderFifo(fn string) {
	f, err := os.OpenFile(fn, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return
	}
	f.Close()
}

// closeWriterFifo closes fifo that may be blocked on open by opening the reader side.
func closeWriterFifo(fn string) {
	f, err := os.OpenFile(fn, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return
	}
	f.Close()
}

func (p *process) fifo(index int) string {
	return filepath.Join(p.dir, p.friendlyName+"-"+fdNames[index])
}
