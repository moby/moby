//go:build !unix && !windows

package launcher

import (
	"os"
	"os/exec"
)

type noopProcessLifetime struct{}

func (noopProcessLifetime) Close() error { return nil }

func startProcess(cmd *exec.Cmd) (processLifetime, <-chan error, error) {
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	return noopProcessLifetime{}, wait, nil
}

func signalProcess(cmd *exec.Cmd, sig os.Signal) error { return cmd.Process.Signal(sig) }

func killProcess(cmd *exec.Cmd) error { return cmd.Process.Kill() }
