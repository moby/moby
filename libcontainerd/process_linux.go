package libcontainerd

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	containerd "github.com/docker/containerd/api/grpc/types"
	"github.com/tonistiigi/fifo"
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

func (p *process) openFifos(terminal bool) (pipe *IOPipe, err error) {
	if err := os.MkdirAll(p.dir, 0700); err != nil {
		return nil, err
	}

	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)

	io := &IOPipe{}

	io.Stdin, err = fifo.OpenFifo(ctx, p.fifo(syscall.Stdin), syscall.O_WRONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			io.Stdin.Close()
		}
	}()

	io.Stdout, err = fifo.OpenFifo(ctx, p.fifo(syscall.Stdout), syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			io.Stdout.Close()
		}
	}()

	if !terminal {
		io.Stderr, err = fifo.OpenFifo(ctx, p.fifo(syscall.Stderr), syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				io.Stderr.Close()
			}
		}()
	} else {
		io.Stderr = ioutil.NopCloser(emptyReader{})
	}

	return io, nil
}

func (p *process) sendCloseStdin() error {
	_, err := p.client.remote.apiClient.UpdateProcess(context.Background(), &containerd.UpdateProcessRequest{
		Id:         p.containerID,
		Pid:        p.friendlyName,
		CloseStdin: true,
	})
	return err
}

func (p *process) closeFifos(io *IOPipe) {
	io.Stdin.Close()
	io.Stdout.Close()
	io.Stderr.Close()
}

type emptyReader struct{}

func (r emptyReader) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (p *process) fifo(index int) string {
	return filepath.Join(p.dir, p.friendlyName+"-"+fdNames[index])
}
