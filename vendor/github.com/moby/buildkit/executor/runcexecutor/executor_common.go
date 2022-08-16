//go:build !linux
// +build !linux

package runcexecutor

import (
	"context"

	runc "github.com/containerd/go-runc"
	"github.com/moby/buildkit/executor"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var unsupportedConsoleError = errors.New("tty for runc is only supported on linux")

func updateRuncFieldsForHostOS(runtime *runc.Runc) {}

func (w *runcExecutor) run(ctx context.Context, id, bundle string, process executor.ProcessInfo, started func()) error {
	if process.Meta.Tty {
		return unsupportedConsoleError
	}
	return w.commonCall(ctx, id, bundle, process, started, func(ctx context.Context, started chan<- int, io runc.IO) error {
		_, err := w.runc.Run(ctx, id, bundle, &runc.CreateOpts{
			NoPivot: w.noPivot,
			Started: started,
			IO:      io,
		})
		return err
	})
}

func (w *runcExecutor) exec(ctx context.Context, id, bundle string, specsProcess *specs.Process, process executor.ProcessInfo, started func()) error {
	if process.Meta.Tty {
		return unsupportedConsoleError
	}
	return w.commonCall(ctx, id, bundle, process, started, func(ctx context.Context, started chan<- int, io runc.IO) error {
		return w.runc.Exec(ctx, id, *specsProcess, &runc.ExecOpts{
			Started: started,
			IO:      io,
		})
	})
}

type runcCall func(ctx context.Context, started chan<- int, io runc.IO) error

// commonCall is the common run/exec logic used for non-linux runtimes. A tty
// is only supported for linux, so this really just handles signal propagation
// to the started runc process.
func (w *runcExecutor) commonCall(ctx context.Context, id, bundle string, process executor.ProcessInfo, started func(), call runcCall) error {
	runcProcess := &startingProcess{
		ready: make(chan struct{}),
	}
	defer runcProcess.Release()

	var eg errgroup.Group
	egCtx, cancel := context.WithCancel(ctx)
	defer eg.Wait()
	defer cancel()

	startedCh := make(chan int, 1)
	eg.Go(func() error {
		return runcProcess.WaitForStart(egCtx, startedCh, started)
	})

	eg.Go(func() error {
		return handleSignals(egCtx, runcProcess, process.Signal)
	})

	return call(ctx, startedCh, &forwardIO{stdin: process.Stdin, stdout: process.Stdout, stderr: process.Stderr})
}
