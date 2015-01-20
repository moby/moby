// +build linux

package native

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/namespaces"
)

func init() {
	reexec.Register(DriverName, initializer)
}

func initializer() {
	runtime.LockOSThread()

	var (
		pipe    = flag.Int("pipe", 0, "sync pipe fd")
		console = flag.String("console", "", "console (pty slave) path")
		root    = flag.String("root", ".", "root path for configuration files")
	)

	flag.Parse()

	var container *libcontainer.Config
	f, err := os.Open(filepath.Join(*root, "container.json"))
	if err != nil {
		writeError(err)
	}

	if err := json.NewDecoder(f).Decode(&container); err != nil {
		f.Close()
		writeError(err)
	}
	f.Close()

	rootfs, err := os.Getwd()
	if err != nil {
		writeError(err)
	}

	if err := namespaces.Init(container, rootfs, *console, os.NewFile(uintptr(*pipe), "child"), flag.Args()); err != nil {
		writeError(err)
	}

	panic("Unreachable")
}

func writeError(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}
