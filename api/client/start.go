package client

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
)

func (cli *DockerCli) forwardAllSignals(cid string) chan os.Signal {
	sigc := make(chan os.Signal, 128)
	signal.CatchAll(sigc)
	go func() {
		for s := range sigc {
			if s == signal.SIGCHLD {
				continue
			}
			var sig string
			for sigStr, sigN := range signal.SignalMap {
				if sigN == s {
					sig = sigStr
					break
				}
			}
			if sig == "" {
				fmt.Fprintf(cli.err, "Unsupported signal: %v. Discarding.\n", s)
				continue
			}

			if err := cli.client.ContainerKill(cid, sig); err != nil {
				logrus.Debugf("Error sending signal: %s", err)
			}
		}
	}()
	return sigc
}

// CmdStart starts one or more containers.
//
// Usage: docker start [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStart(args ...string) error {
	cmd := Cli.Subcmd("start", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["start"].Description, true)
	attach := cmd.Bool([]string{"a", "-attach"}, false, "Attach STDOUT/STDERR and forward signals")
	openStdin := cmd.Bool([]string{"i", "-interactive"}, false, "Attach container's STDIN")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *attach || *openStdin {
		// We're going to attach to a container.
		// 1. Ensure we only have one container.
		if cmd.NArg() > 1 {
			return fmt.Errorf("You cannot start and attach multiple containers at once.")
		}

		// 2. Attach to the container.
		containerID := cmd.Arg(0)
		c, err := cli.client.ContainerInspect(containerID)
		if err != nil {
			return err
		}

		if !c.Config.Tty {
			sigc := cli.forwardAllSignals(containerID)
			defer signal.StopCatch(sigc)
		}

		options := types.ContainerAttachOptions{
			ContainerID: containerID,
			Stream:      true,
			Stdin:       *openStdin && c.Config.OpenStdin,
			Stdout:      true,
			Stderr:      true,
		}

		var in io.ReadCloser
		if options.Stdin {
			in = cli.in
		}

		resp, err := cli.client.ContainerAttach(options)
		if err != nil {
			return err
		}
		defer resp.Close()

		cErr := promise.Go(func() error {
			return cli.holdHijackedConnection(c.Config.Tty, in, cli.out, cli.err, resp)
		})

		// 3. Start the container.
		if err := cli.client.ContainerStart(containerID); err != nil {
			return err
		}

		// 4. Wait for attachment to break.
		if c.Config.Tty && cli.isTerminalOut {
			if err := cli.monitorTtySize(containerID, false); err != nil {
				fmt.Fprintf(cli.err, "Error monitoring TTY size: %s\n", err)
			}
		}
		if attchErr := <-cErr; attchErr != nil {
			return attchErr
		}
		_, status, err := getExitCode(cli, containerID)
		if err != nil {
			return err
		}
		if status != 0 {
			return Cli.StatusError{StatusCode: status}
		}
	} else {
		// We're not going to attach to anything.
		// Start as many containers as we want.
		return cli.startContainersWithoutAttachments(cmd.Args())
	}

	return nil
}

func (cli *DockerCli) startContainersWithoutAttachments(containerIDs []string) error {
	var failedContainers []string
	for _, containerID := range containerIDs {
		if err := cli.client.ContainerStart(containerID); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			failedContainers = append(failedContainers, containerID)
		} else {
			fmt.Fprintf(cli.out, "%s\n", containerID)
		}
	}

	if len(failedContainers) > 0 {
		return fmt.Errorf("Error: failed to start containers: %v", strings.Join(failedContainers, ", "))
	}
	return nil
}
