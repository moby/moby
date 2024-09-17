//go:build !windows

package main

import (
	"context"
	"io"

	"github.com/containerd/log"
)

func runDaemon(ctx context.Context, cli *daemonCLI) error {
	return cli.start(ctx)
}

func initLogging(_, stderr io.Writer) {
	log.L.Logger.SetOutput(stderr)
}
