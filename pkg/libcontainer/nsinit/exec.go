package main

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/system"
	"github.com/dotcloud/docker/pkg/term"
	"io"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func execCommand(container *libcontainer.Container) (pid int, err error) {
	master, console, err := createMasterAndConsole()
	if err != nil {
		return -1, err
	}

	// we need CLONE_VFORK so we can wait on the child
	flag := uintptr(getNamespaceFlags(container.Namespaces) | CLONE_VFORK)

	command := exec.Command("nsinit", console)
	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: flag,
	}

	if err := command.Start(); err != nil {
		return -1, err
	}
	pid = command.Process.Pid

	go func() {
		if _, err := io.Copy(os.Stdout, master); err != nil {
			log.Println(err)
		}
	}()

	go func() {
		if _, err := io.Copy(master, os.Stdin); err != nil {
			log.Println(err)
		}
	}()

	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return -1, err
	}
	if err := term.SetWinsize(master.Fd(), ws); err != nil {
		return -1, err
	}
	state, err := term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		command.Process.Kill()
		return -1, err
	}
	defer term.RestoreTerminal(os.Stdin.Fd(), state)

	if err := command.Wait(); err != nil {
		return pid, err
	}
	return pid, nil
}

func createMasterAndConsole() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}

	console, err := system.Ptsname(master)
	if err != nil {
		return nil, "", err
	}

	if err := system.Unlockpt(master); err != nil {
		return nil, "", err
	}
	return master, console, nil
}
