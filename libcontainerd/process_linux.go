package libcontainerd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/docker/docker/pkg/ioutils"
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
	stdinf, err := os.OpenFile(p.fifo(syscall.Stdin), syscall.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	io.Stdout = openReaderFromFifo(p.fifo(syscall.Stdout))
	if !terminal {
		io.Stderr = openReaderFromFifo(p.fifo(syscall.Stderr))
	} else {
		io.Stderr = emptyReader{}
	}

	io.Stdin = ioutils.NewWriteCloserWrapper(stdinf, func() error {
		stdinf.Close()
		_, err := p.client.remote.apiClient.UpdateProcess(context.Background(), &containerd.UpdateProcessRequest{
			Id:         p.containerID,
			Pid:        p.friendlyName,
			CloseStdin: true,
		})
		return err
	})

	return io, nil
}

func (p *process) closeFifos(io *IOPipe) {
	io.Stdin.Close()
	closeReaderFifo(p.fifo(syscall.Stdout))
	closeReaderFifo(p.fifo(syscall.Stderr))
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

func (p *process) fifo(index int) string {
	return filepath.Join(p.dir, p.friendlyName+"-"+fdNames[index])
}
