//go:build !linux
// +build !linux

package runcexecutor

import (
	"context"

	runc "github.com/containerd/go-runc"
	"github.com/moby/buildkit/executor"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

var unsupportedConsoleError = errors.New("tty for runc is only supported on linux")

func updateRuncFieldsForHostOS(runtime *runc.Runc) {}

func (w *runcExecutor) run(ctx context.Context, id, bundle string, process executor.ProcessInfo) error {
	if process.Meta.Tty {
		return unsupportedConsoleError
	}
	_, err := w.runc.Run(ctx, id, bundle, &runc.CreateOpts{
		IO:      &forwardIO{stdin: process.Stdin, stdout: process.Stdout, stderr: process.Stderr},
		NoPivot: w.noPivot,
	})
	return err
}

func (w *runcExecutor) exec(ctx context.Context, id, bundle string, specsProcess *specs.Process, process executor.ProcessInfo) error {
	if process.Meta.Tty {
		return unsupportedConsoleError
	}
	return w.runc.Exec(ctx, id, *specsProcess, &runc.ExecOpts{
		IO: &forwardIO{stdin: process.Stdin, stdout: process.Stdout, stderr: process.Stderr},
	})
}
