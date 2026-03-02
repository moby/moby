//go:build unix && !linux

package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/moby/sys/reexec"
	"golang.org/x/sys/unix"
)

const (
	gitCmd = "umask-git"
)

func init() {
	reexec.Register(gitCmd, gitMain)
}

func gitMain() {
	// Need standard user umask for git process.
	unix.Umask(0022)

	// Reexec git command
	cmd := exec.CommandContext(context.Background(), os.Args[1], os.Args[2:]...) //nolint:gosec // reexec
	cmd.SysProcAttr = &reexecSysProcAttr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Forward all signals
	sigc := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigc)
	go func() {
		for {
			select {
			case sig := <-sigc:
				if cmd.Process == nil {
					continue
				}
				switch sig {
				case unix.SIGINT, unix.SIGTERM, unix.SIGKILL:
					_ = unix.Kill(-cmd.Process.Pid, sig.(unix.Signal))
				default:
					_ = cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()

	err := cmd.Run()
	close(done)
	if err != nil {
		exiterr := &exec.ExitError{}
		if errors.As(err, &exiterr) {
			switch status := exiterr.Sys().(type) {
			case unix.WaitStatus:
				os.Exit(status.ExitStatus())
			case syscall.WaitStatus:
				os.Exit(status.ExitStatus())
			}
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func runWithStandardUmask(ctx context.Context, cmd *exec.Cmd) error {
	cmd.Path = reexec.Self()
	cmd.Args = append([]string{gitCmd}, cmd.Args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	waitDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = unix.Kill(-cmd.Process.Pid, unix.SIGTERM)
			go func() {
				select {
				case <-waitDone:
				case <-time.After(10 * time.Second):
					_ = unix.Kill(-cmd.Process.Pid, unix.SIGKILL)
				}
			}()
		case <-waitDone:
		}
	}()
	err := cmd.Wait()
	close(waitDone)
	return err
}
