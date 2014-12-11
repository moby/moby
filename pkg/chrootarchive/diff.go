package chrootarchive

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
)

func applyLayer() {
	runtime.LockOSThread()
	flag.Parse()

	if err := chroot(flag.Arg(0)); err != nil {
		fatal(err)
	}
	// We need to be able to set any perms
	oldmask := syscall.Umask(0)
	defer syscall.Umask(oldmask)
	tmpDir, err := ioutil.TempDir("/", "temp-docker-extract")
	if err != nil {
		fatal(err)
	}
	os.Setenv("TMPDIR", tmpDir)
	err = archive.UnpackLayer("/", os.Stdin)
	os.RemoveAll(tmpDir)
	if err != nil {
		fatal(err)
	}
	os.RemoveAll(tmpDir)
	flush(os.Stdin)
	os.Exit(0)
}

func ApplyLayer(dest string, layer archive.ArchiveReader) error {
	dest = filepath.Clean(dest)
	decompressed, err := archive.DecompressStream(layer)
	if err != nil {
		return err
	}
	defer func() {
		if c, ok := decompressed.(io.Closer); ok {
			c.Close()
		}
	}()
	cmd := reexec.Command("docker-applyLayer", dest)
	cmd.Stdin = decompressed
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ApplyLayer %s %s", err, out)
	}
	return nil
}
