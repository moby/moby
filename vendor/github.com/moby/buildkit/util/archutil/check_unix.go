//go:build !windows
// +build !windows

package archutil

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
)

func withChroot(cmd *exec.Cmd, dir string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot: dir,
	}
}

func check(arch, bin string) (string, error) {
	tmpdir, err := ioutil.TempDir("", "qemu-check")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpdir)
	pp := filepath.Join(tmpdir, "check")

	r, err := gzip.NewReader(bytes.NewReader([]byte(bin)))
	if err != nil {
		return "", err
	}
	defer r.Close()

	f, err := os.OpenFile(pp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	cmd := exec.Command("/check")
	withChroot(cmd, tmpdir)
	err = cmd.Run()
	if arch != "amd64" {
		return "", err
	}

	// special handling for amd64. Exit code is 64 + amd64 variant
	if err == nil {
		return "", errors.Errorf("invalid zero exit code")
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		switch exitError.ExitCode() {
		case 65:
			return "v1", nil
		case 66:
			return "v2", nil
		}
	}
	return "", err
}
