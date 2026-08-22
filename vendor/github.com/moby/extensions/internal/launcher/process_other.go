//go:build !windows

package launcher

import "os/exec"

type noopProcessLifetime struct{}

func (noopProcessLifetime) Close() error { return nil }

func startProcess(cmd *exec.Cmd) (processLifetime, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return noopProcessLifetime{}, nil
}
