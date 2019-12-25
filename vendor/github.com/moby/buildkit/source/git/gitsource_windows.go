// +build windows

package git

import (
	"context"
	"os/exec"
)

func runProcessGroup(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	waitDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
		case <-waitDone:
		}
	}()
	return cmd.Wait()
}
