package chrootarchive

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
)

func applyLayer() {
	runtime.LockOSThread()
	flag.Parse()

	if err := syscall.Chroot(flag.Arg(0)); err != nil {
		fatal(err)
	}
	if err := syscall.Chdir("/"); err != nil {
		fatal(err)
	}
	if err := archive.ApplyLayer("/", os.Stdin); err != nil {
		fatal(err)
	}
	os.Exit(0)
}

func ApplyLayer(dest string, layer archive.ArchiveReader) error {
	cmd := reexec.Command("docker-applyLayer", dest)
	cmd.Stdin = layer
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ApplyLayer %s %s", err, out)
	}
	return nil
}
