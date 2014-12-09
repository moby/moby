package chrootarchive

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
)

func untar() {
	runtime.LockOSThread()
	flag.Parse()

	if err := syscall.Chroot(flag.Arg(0)); err != nil {
		fatal(err)
	}
	if err := syscall.Chdir("/"); err != nil {
		fatal(err)
	}
	options := new(archive.TarOptions)
	dec := json.NewDecoder(strings.NewReader(flag.Arg(1)))
	if err := dec.Decode(options); err != nil {
		fatal(err)
	}
	if err := archive.Untar(os.Stdin, "/", options); err != nil {
		fatal(err)
	}
	os.Exit(0)
}

var (
	chrootArchiver = &archive.Archiver{Untar}
)

func Untar(archive io.Reader, dest string, options *archive.TarOptions) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(options); err != nil {
		return fmt.Errorf("Untar json encode: %v", err)
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		if err := os.MkdirAll(dest, 0777); err != nil {
			return err
		}
	}

	cmd := reexec.Command("docker-untar", dest, buf.String())
	cmd.Stdin = archive
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Untar %s %s", err, out)
	}
	return nil
}

func TarUntar(src, dst string) error {
	return chrootArchiver.TarUntar(src, dst)
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
func CopyWithTar(src, dst string) error {
	return chrootArchiver.CopyWithTar(src, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
//
// If `dst` ends with a trailing slash '/', the final destination path
// will be `dst/base(src)`.
func CopyFileWithTar(src, dst string) (err error) {
	return chrootArchiver.CopyFileWithTar(src, dst)
}

// UntarPath is a convenience function which looks for an archive
// at filesystem path `src`, and unpacks it at `dst`.
func UntarPath(src, dst string) error {
	return chrootArchiver.UntarPath(src, dst)
}
