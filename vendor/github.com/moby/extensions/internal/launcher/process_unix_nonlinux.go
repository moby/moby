//go:build unix && !linux

package launcher

import "os/exec"

func startProcess(cmd *exec.Cmd) (processLifetime, <-chan error, error) {
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	wait := make(chan error, 1)
	go func() { wait <- reapProcess(cmd) }()
	return processGroupLifetime{}, wait, nil
}
