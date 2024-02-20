//go:build !windows

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/containerd/log"
)

func runDaemon(opts *daemonOptions) error {
	cli, err := NewDaemonCli(opts)
	if err != nil {
		return err
	}
	if opts.Validate {
		// If config wasn't OK we wouldn't have made it this far.
		_, _ = fmt.Fprintln(os.Stderr, "configuration OK")
		return nil
	}
	return cli.start()
}

func initLogging(_, stderr io.Writer) {
	log.L.Logger.SetOutput(stderr)
}
