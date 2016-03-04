// +build linux

package native

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/docker/docker/pkg/reexec"
	"github.com/opencontainers/runc/libcontainer"
)

func init() {
	reexec.Register(DriverName, initializer)
}

func fatal(err error) {
	// return proper unix error codes
	if exerr, ok := err.(*exec.Error); ok {
		switch exerr.Err {
		case os.ErrPermission:
			fmt.Fprintln(os.Stderr, err)
			os.Exit(126)
		case exec.ErrNotFound:
			fmt.Fprintln(os.Stderr, err)
			os.Exit(127)
		default:
			if os.IsNotExist(exerr.Err) {
				fmt.Fprintf(os.Stderr, "exec: %s: %v\n", strconv.Quote(exerr.Name), os.ErrNotExist)
				os.Exit(127)
			}
		}
	}
	if lerr, ok := err.(libcontainer.Error); ok {
		lerr.Detail(os.Stderr)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func initializer() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
	factory, err := libcontainer.New("")
	if err != nil {
		fatal(err)
	}
	if err := factory.StartInitialization(); err != nil {
		fatal(err)
	}

	panic("unreachable")
}

func writeError(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}
