package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"

	"github.com/codegangsta/cli"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/namespaces"
)

var execCommand = cli.Command{
	Name:   "exec",
	Usage:  "execute a new command inside a container",
	Action: execAction,
}

func execAction(context *cli.Context) {
	var nspid, exitCode int

	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	if nspid, err = readPid(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("unable to read pid: %s", err)
	}

	if nspid > 0 {
		err = namespaces.ExecIn(container, nspid, []string(context.Args()))
	} else {
		term := namespaces.NewTerminal(os.Stdin, os.Stdout, os.Stderr, container.Tty)
		exitCode, err = startContainer(container, term, dataPath, []string(context.Args()))
	}

	if err != nil {
		log.Fatalf("failed to exec: %s", err)
	}

	os.Exit(exitCode)
}

// startContainer starts the container. Returns the exit status or -1 and an
// error.
//
// Signals sent to the current process will be forwarded to container.
func startContainer(container *libcontainer.Container, term namespaces.Terminal, dataPath string, args []string) (int, error) {
	var (
		cmd  *exec.Cmd
		sigc = make(chan os.Signal, 10)
	)

	signal.Notify(sigc)

	createCommand := func(container *libcontainer.Container, console, rootfs, dataPath, init string, pipe *os.File, args []string) *exec.Cmd {
		cmd = namespaces.DefaultCreateCommand(container, console, rootfs, dataPath, init, pipe, args)
		if logPath != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("log=%s", logPath))
		}
		return cmd
	}

	startCallback := func() {
		go func() {
			for sig := range sigc {
				cmd.Process.Signal(sig)
			}
		}()
	}

	return namespaces.Exec(container, term, "", dataPath, args, createCommand, startCallback)
}
