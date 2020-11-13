package runcexecutor

import (
	"context"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/containerd/console"
	runc "github.com/containerd/go-runc"
	"github.com/docker/docker/pkg/signal"
	"github.com/moby/buildkit/executor"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func updateRuncFieldsForHostOS(runtime *runc.Runc) {
	// PdeathSignal only supported on unix platforms
	runtime.PdeathSignal = syscall.SIGKILL // this can still leak the process
}

func (w *runcExecutor) run(ctx context.Context, id, bundle string, process executor.ProcessInfo) error {
	return w.callWithIO(ctx, id, bundle, process, func(ctx context.Context, started chan<- int, io runc.IO) error {
		_, err := w.runc.Run(ctx, id, bundle, &runc.CreateOpts{
			NoPivot: w.noPivot,
			Started: started,
			IO:      io,
		})
		return err
	})
}

func (w *runcExecutor) exec(ctx context.Context, id, bundle string, specsProcess *specs.Process, process executor.ProcessInfo) error {
	return w.callWithIO(ctx, id, bundle, process, func(ctx context.Context, started chan<- int, io runc.IO) error {
		return w.runc.Exec(ctx, id, *specsProcess, &runc.ExecOpts{
			Started: started,
			IO:      io,
		})
	})
}

type runcCall func(ctx context.Context, started chan<- int, io runc.IO) error

func (w *runcExecutor) callWithIO(ctx context.Context, id, bundle string, process executor.ProcessInfo, call runcCall) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if !process.Meta.Tty {
		return call(ctx, nil, &forwardIO{stdin: process.Stdin, stdout: process.Stdout, stderr: process.Stderr})
	}

	ptm, ptsName, err := console.NewPty()
	if err != nil {
		return err
	}

	pts, err := os.OpenFile(ptsName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		ptm.Close()
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	defer func() {
		if process.Stdin != nil {
			process.Stdin.Close()
		}
		pts.Close()
		ptm.Close()
		cancel() // this will shutdown resize loop
		err := eg.Wait()
		if err != nil {
			logrus.Warningf("error while shutting down tty io: %s", err)
		}
	}()

	if process.Stdin != nil {
		eg.Go(func() error {
			_, err := io.Copy(ptm, process.Stdin)
			// stdin might be a pipe, so this is like EOF
			if errors.Is(err, io.ErrClosedPipe) {
				return nil
			}
			return err
		})
	}

	if process.Stdout != nil {
		eg.Go(func() error {
			_, err := io.Copy(process.Stdout, ptm)
			// ignore `read /dev/ptmx: input/output error` when ptm is closed
			var ptmClosedError *os.PathError
			if errors.As(err, &ptmClosedError) {
				if ptmClosedError.Op == "read" &&
					ptmClosedError.Path == "/dev/ptmx" &&
					ptmClosedError.Err == syscall.EIO {
					return nil
				}
			}
			return err
		})
	}

	started := make(chan int, 1)

	eg.Go(func() error {
		startedCtx, timeout := context.WithTimeout(ctx, 10*time.Second)
		defer timeout()
		var runcProcess *os.Process
		select {
		case <-startedCtx.Done():
			return errors.New("runc started message never received")
		case pid, ok := <-started:
			if !ok {
				return errors.New("runc process failed to send pid")
			}
			runcProcess, err = os.FindProcess(pid)
			if err != nil {
				return errors.Wrapf(err, "unable to find runc process for pid %d", pid)
			}
			defer runcProcess.Release()
		}

		for {
			select {
			case <-ctx.Done():
				return nil
			case resize := <-process.Resize:
				err = ptm.Resize(console.WinSize{
					Height: uint16(resize.Rows),
					Width:  uint16(resize.Cols),
				})
				if err != nil {
					logrus.Errorf("failed to resize ptm: %s", err)
				}
				err = runcProcess.Signal(signal.SIGWINCH)
				if err != nil {
					logrus.Errorf("failed to send SIGWINCH to process: %s", err)
				}
			}
		}
	})

	runcIO := &forwardIO{}
	if process.Stdin != nil {
		runcIO.stdin = pts
	}
	if process.Stdout != nil {
		runcIO.stdout = pts
	}
	if process.Stderr != nil {
		runcIO.stderr = pts
	}

	return call(ctx, started, runcIO)
}
