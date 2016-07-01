package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	gosignal "os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
)

func (cli *DockerCli) resizeTty(ctx context.Context, id string, isExec bool) {
	height, width := cli.GetTtySize()
	cli.ResizeTtyTo(ctx, id, height, width, isExec)
}

// ResizeTtyTo resizes tty to specific height and width
// TODO: this can be unexported again once all container related commands move to package container
func (cli *DockerCli) ResizeTtyTo(ctx context.Context, id string, height, width int, isExec bool) {
	if height == 0 && width == 0 {
		return
	}

	options := types.ResizeOptions{
		Height: height,
		Width:  width,
	}

	var err error
	if isExec {
		err = cli.client.ContainerExecResize(ctx, id, options)
	} else {
		err = cli.client.ContainerResize(ctx, id, options)
	}

	if err != nil {
		logrus.Debugf("Error resize: %s", err)
	}
}

// getExecExitCode perform an inspect on the exec command. It returns
// the running state and the exit code.
func (cli *DockerCli) getExecExitCode(ctx context.Context, execID string) (bool, int, error) {
	resp, err := cli.client.ContainerExecInspect(ctx, execID)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if err != client.ErrConnectionFailed {
			return false, -1, err
		}
		return false, -1, nil
	}

	return resp.Running, resp.ExitCode, nil
}

// MonitorTtySize updates the container tty size when the terminal tty changes size
func (cli *DockerCli) MonitorTtySize(ctx context.Context, id string, isExec bool) error {
	cli.resizeTty(ctx, id, isExec)

	if runtime.GOOS == "windows" {
		go func() {
			prevH, prevW := cli.GetTtySize()
			for {
				time.Sleep(time.Millisecond * 250)
				h, w := cli.GetTtySize()

				if prevW != w || prevH != h {
					cli.resizeTty(ctx, id, isExec)
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
				cli.resizeTty(ctx, id, isExec)
			}
		}()
	}
	return nil
}

// GetTtySize returns the height and width in characters of the tty
func (cli *DockerCli) GetTtySize() (int, int) {
	if !cli.isTerminalOut {
		return 0, 0
	}
	ws, err := term.GetWinsize(cli.outFd)
	if err != nil {
		logrus.Debugf("Error getting size: %s", err)
		if ws == nil {
			return 0, 0
		}
	}
	return int(ws.Height), int(ws.Width)
}

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	tmpFile, err := ioutil.TempFile(filepath.Dir(outfile), ".docker_temp_")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, r)
	tmpFile.Close()

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err = os.Rename(tmpPath, outfile); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// ForwardAllSignals forwards signals to the container
// TODO: this can be unexported again once all container commands are under
// api/client/container
func (cli *DockerCli) ForwardAllSignals(ctx context.Context, cid string) chan os.Signal {
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
				fmt.Fprintf(cli.err, "Unsupported signal: %v. Discarding.\n", s)
				continue
			}

			if err := cli.client.ContainerKill(ctx, cid, sig); err != nil {
				logrus.Debugf("Error sending signal: %s", err)
			}
		}
	}()
	return sigc
}

// capitalizeFirst capitalizes the first character of string
func capitalizeFirst(s string) string {
	switch l := len(s); l {
	case 0:
		return s
	case 1:
		return strings.ToLower(s)
	default:
		return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
	}
}

// PrettyPrint outputs arbitrary data for human formatted output by uppercasing the first letter.
func PrettyPrint(i interface{}) string {
	switch t := i.(type) {
	case nil:
		return "None"
	case string:
		return capitalizeFirst(t)
	default:
		return capitalizeFirst(fmt.Sprintf("%s", t))
	}
}
