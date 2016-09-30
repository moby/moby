package container

import (
	"fmt"
	"os"
	gosignal "os/signal"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/signal"
	"golang.org/x/net/context"
)

// resizeTtyTo resizes tty to specific height and width
func resizeTtyTo(ctx context.Context, client client.ContainerAPIClient, id string, height, width uint, isExec bool) {
	if height == 0 && width == 0 {
		return
	}

	options := types.ResizeOptions{
		Height: height,
		Width:  width,
	}

	var err error
	if isExec {
		err = client.ContainerExecResize(ctx, id, options)
	} else {
		err = client.ContainerResize(ctx, id, options)
	}

	if err != nil {
		logrus.Debugf("Error resize: %s", err)
	}
}

// MonitorTtySize updates the container tty size when the terminal tty changes size
func MonitorTtySize(ctx context.Context, cli *command.DockerCli, id string, isExec bool) error {
	resizeTty := func() {
		height, width := cli.Out().GetTtySize()
		resizeTtyTo(ctx, cli.Client(), id, height, width, isExec)
	}

	resizeTty()

	if runtime.GOOS == "windows" {
		go func() {
			prevH, prevW := cli.Out().GetTtySize()
			for {
				time.Sleep(time.Millisecond * 250)
				h, w := cli.Out().GetTtySize()

				if prevW != w || prevH != h {
					resizeTty()
				}
				prevH = h
				prevW = w
			}
		}()
	} else {
		sigchan := make(chan os.Signal, 1)
		gosignal.Notify(sigchan, signal.SIGWINCH)
		go func() {
			for range sigchan {
				resizeTty()
			}
		}()
	}
	return nil
}

// ForwardAllSignals forwards signals to the container
func ForwardAllSignals(ctx context.Context, cli *command.DockerCli, cid string) chan os.Signal {
	sigc := make(chan os.Signal, 128)
	signal.CatchAll(sigc)
	go func() {
		for s := range sigc {
			if s == signal.SIGCHLD || s == signal.SIGPIPE {
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
				fmt.Fprintf(cli.Err(), "Unsupported signal: %v. Discarding.\n", s)
				continue
			}

			if err := cli.Client().ContainerKill(ctx, cid, sig); err != nil {
				logrus.Debugf("Error sending signal: %s", err)
			}
		}
	}()
	return sigc
}
