//go:build !linux
// +build !linux

package runcexecutor

import (
	"context"

	runc "github.com/containerd/go-runc"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/util/bklog"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var errUnsupportedConsole = errors.New("tty for runc is only supported on linux")

func updateRuncFieldsForHostOS(runtime *runc.Runc) {}

func (w *runcExecutor) run(ctx context.Context, id, bundle string, process executor.ProcessInfo, started func(), keep bool) error {
	if process.Meta.Tty {
		return errUnsupportedConsole
	}
	extraArgs := []string{}
	if keep {
		extraArgs = append(extraArgs, "--keep")
	}
	killer := newRunProcKiller(w.runc, id)
	return w.commonCall(ctx, id, bundle, process, started, killer, func(ctx context.Context, started chan<- int, io runc.IO, pidfile string) error {
		_, err := w.runc.Run(ctx, id, bundle, &runc.CreateOpts{
			NoPivot:   w.noPivot,
			Started:   started,
			IO:        io,
			ExtraArgs: extraArgs,
		})
		return err
	})
}

func (w *runcExecutor) exec(ctx context.Context, id, bundle string, specsProcess *specs.Process, process executor.ProcessInfo, started func()) error {
	if process.Meta.Tty {
		return errUnsupportedConsole
	}

	killer, err := newExecProcKiller(w.runc, id)
	if err != nil {
		return errors.Wrap(err, "failed to initialize process killer")
	}
	defer killer.Cleanup()

	return w.commonCall(ctx, id, bundle, process, started, killer, func(ctx context.Context, started chan<- int, io runc.IO, pidfile string) error {
		return w.runc.Exec(ctx, id, *specsProcess, &runc.ExecOpts{
			Started: started,
			IO:      io,
			PidFile: pidfile,
		})
	})
}

type runcCall func(ctx context.Context, started chan<- int, io runc.IO, pidfile string) error

// commonCall is the common run/exec logic used for non-linux runtimes. A tty
// is only supported for linux, so this really just handles signal propagation
// to the started runc process.
func (w *runcExecutor) commonCall(ctx context.Context, id, bundle string, process executor.ProcessInfo, started func(), killer procKiller, call runcCall) error {
	runcProcess, ctx := runcProcessHandle(ctx, killer)
	defer runcProcess.Release()

	eg, ctx := errgroup.WithContext(ctx)
	defer func() {
		if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			bklog.G(ctx).Errorf("runc process monitoring error: %s", err)
		}
	}()
	defer runcProcess.Shutdown()

	startedCh := make(chan int, 1)
	eg.Go(func() error {
		return runcProcess.WaitForStart(ctx, startedCh, started)
	})

	eg.Go(func() error {
		return handleSignals(ctx, runcProcess, process.Signal)
	})

	return call(ctx, startedCh, &forwardIO{stdin: process.Stdin, stdout: process.Stdout, stderr: process.Stderr}, killer.pidfile)
}
