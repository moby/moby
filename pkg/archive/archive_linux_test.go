package archive // import "github.com/docker/docker/pkg/archive"

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/system"
	rsystem "github.com/opencontainers/runc/libcontainer/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"gotest.tools/assert"
	"gotest.tools/skip"
)

// setupOverlayTestDir creates files in a directory with overlay whiteouts
// Tree layout
// .
// ├── d1     # opaque, 0700
// │   └── f1 # empty file, 0600
// ├── d2     # opaque, 0750
// │   └── f1 # empty file, 0660
// └── d3     # 0700
//     └── f1 # whiteout, 0644
func setupOverlayTestDir(t *testing.T, src string) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	skip.If(t, rsystem.RunningInUserNS(), "skipping test that requires initial userns (trusted.overlay.opaque xattr cannot be set in userns, even with Ubuntu kernel)")
	// Create opaque directory containing single file and permission 0700
	err := os.Mkdir(filepath.Join(src, "d1"), 0700)
	assert.NilError(t, err)

	err = system.Lsetxattr(filepath.Join(src, "d1"), "trusted.overlay.opaque", []byte("y"), 0)
	assert.NilError(t, err)

	err = ioutil.WriteFile(filepath.Join(src, "d1", "f1"), []byte{}, 0600)
	assert.NilError(t, err)

	// Create another opaque directory containing single file but with permission 0750
	err = os.Mkdir(filepath.Join(src, "d2"), 0750)
	assert.NilError(t, err)

	err = system.Lsetxattr(filepath.Join(src, "d2"), "trusted.overlay.opaque", []byte("y"), 0)
	assert.NilError(t, err)

	err = ioutil.WriteFile(filepath.Join(src, "d2", "f1"), []byte{}, 0660)
	assert.NilError(t, err)

	// Create regular directory with deleted file
	err = os.Mkdir(filepath.Join(src, "d3"), 0700)
	assert.NilError(t, err)

	err = system.Mknod(filepath.Join(src, "d3", "f1"), unix.S_IFCHR, 0)
	assert.NilError(t, err)
}

func checkOpaqueness(t *testing.T, path string, opaque string) {
	xattrOpaque, err := system.Lgetxattr(path, "trusted.overlay.opaque")
	assert.NilError(t, err)

	if string(xattrOpaque) != opaque {
		t.Fatalf("Unexpected opaque value: %q, expected %q", string(xattrOpaque), opaque)
	}

}

func checkOverlayWhiteout(t *testing.T, path string) {
	stat, err := os.Stat(path)
	assert.NilError(t, err)

	statT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("Unexpected type: %t, expected *syscall.Stat_t", stat.Sys())
	}
	if statT.Rdev != 0 {
		t.Fatalf("Non-zero device number for whiteout")
	}
}

func checkFileMode(t *testing.T, path string, perm os.FileMode) {
	stat, err := os.Stat(path)
	assert.NilError(t, err)

	if stat.Mode() != perm {
		t.Fatalf("Unexpected file mode for %s: %o, expected %o", path, stat.Mode(), perm)
	}
}

func TestOverlayTarUntar(t *testing.T) {
	oldmask, err := system.Umask(0)
	assert.NilError(t, err)
	defer system.Umask(oldmask)

	src, err := ioutil.TempDir("", "docker-test-overlay-tar-src")
	assert.NilError(t, err)
	defer os.RemoveAll(src)

	setupOverlayTestDir(t, src)

	dst, err := ioutil.TempDir("", "docker-test-overlay-tar-dst")
	assert.NilError(t, err)
	defer os.RemoveAll(dst)

	options := &TarOptions{
		Compression:    Uncompressed,
		WhiteoutFormat: OverlayWhiteoutFormat,
	}
	archive, err := TarWithOptions(src, options)
	assert.NilError(t, err)
	defer archive.Close()

	err = Untar(archive, dst, options)
	assert.NilError(t, err)

	checkFileMode(t, filepath.Join(dst, "d1"), 0700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d2"), 0750|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d3"), 0700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", "f1"), 0600)
	checkFileMode(t, filepath.Join(dst, "d2", "f1"), 0660)
	checkFileMode(t, filepath.Join(dst, "d3", "f1"), os.ModeCharDevice|os.ModeDevice)

	checkOpaqueness(t, filepath.Join(dst, "d1"), "y")
	checkOpaqueness(t, filepath.Join(dst, "d2"), "y")
	checkOpaqueness(t, filepath.Join(dst, "d3"), "")
	checkOverlayWhiteout(t, filepath.Join(dst, "d3", "f1"))
}

func TestOverlayTarAUFSUntar(t *testing.T) {
	oldmask, err := system.Umask(0)
	assert.NilError(t, err)
	defer system.Umask(oldmask)

	src, err := ioutil.TempDir("", "docker-test-overlay-tar-src")
	assert.NilError(t, err)
	defer os.RemoveAll(src)

	setupOverlayTestDir(t, src)

	dst, err := ioutil.TempDir("", "docker-test-overlay-tar-dst")
	assert.NilError(t, err)
	defer os.RemoveAll(dst)

	archive, err := TarWithOptions(src, &TarOptions{
		Compression:    Uncompressed,
		WhiteoutFormat: OverlayWhiteoutFormat,
	})
	assert.NilError(t, err)
	defer archive.Close()

	err = Untar(archive, dst, &TarOptions{
		Compression:    Uncompressed,
		WhiteoutFormat: AUFSWhiteoutFormat,
	})
	assert.NilError(t, err)

	checkFileMode(t, filepath.Join(dst, "d1"), 0700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", WhiteoutOpaqueDir), 0700)
	checkFileMode(t, filepath.Join(dst, "d2"), 0750|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d2", WhiteoutOpaqueDir), 0750)
	checkFileMode(t, filepath.Join(dst, "d3"), 0700|os.ModeDir)
	checkFileMode(t, filepath.Join(dst, "d1", "f1"), 0600)
	checkFileMode(t, filepath.Join(dst, "d2", "f1"), 0660)
	checkFileMode(t, filepath.Join(dst, "d3", WhiteoutPrefix+"f1"), 0600)
}

func unshareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Geteuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getegid(),
				Size:        1,
			},
		},
	}
}

const (
	reexecSupportsUserNSOverlay = "docker-test-supports-userns-overlay"
	reexecMknodChar0            = "docker-test-userns-mknod-char0"
	reexecSetOpaque             = "docker-test-userns-set-opaque"
)

func supportsOverlay(dir string) error {
	lower := filepath.Join(dir, "l")
	upper := filepath.Join(dir, "u")
	work := filepath.Join(dir, "w")
	merged := filepath.Join(dir, "m")
	for _, s := range []string{lower, upper, work, merged} {
		if err := os.MkdirAll(s, 0700); err != nil {
			return err
		}
	}
	mOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	if err := syscall.Mount("overlay", merged, "overlay", uintptr(0), mOpts); err != nil {
		return errors.Wrapf(err, "failed to mount overlay (%s) on %s", mOpts, merged)
	}
	if err := syscall.Unmount(merged, 0); err != nil {
		return errors.Wrapf(err, "failed to unmount %s", merged)
	}
	return nil
}

// supportsUserNSOverlay returns nil error if overlay is supported in userns.
// Only Ubuntu and a few distros support overlay in userns (by patching the kernel).
// https://lists.ubuntu.com/archives/kernel-team/2014-February/038091.html
// As of kernel 4.19, the patch is not merged to the upstream.
func supportsUserNSOverlay() error {
	tmp, err := ioutil.TempDir("", "docker-test-supports-userns-overlay")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	cmd := reexec.Command(reexecSupportsUserNSOverlay, tmp)
	unshareCmd(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "output: %q", string(out))
	}
	return nil
}

// isOpaque returns nil error if the dir has trusted.overlay.opaque=y.
// isOpaque needs to be called in the initial userns.
func isOpaque(dir string) error {
	xattrOpaque, err := system.Lgetxattr(dir, "trusted.overlay.opaque")
	if err != nil {
		return errors.Wrapf(err, "failed to read opaque flag of %s", dir)
	}
	if string(xattrOpaque) != "y" {
		return errors.Errorf("expected \"y\", got %q", string(xattrOpaque))
	}
	return nil
}

func TestReexecUserNSOverlayWhiteoutConverter(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	skip.If(t, rsystem.RunningInUserNS(), "skipping test that requires initial userns")
	if err := supportsUserNSOverlay(); err != nil {
		t.Skipf("skipping test that requires kernel support for overlay-in-userns: %v", err)
	}
	tmp, err := ioutil.TempDir("", "docker-test-userns-overlay")
	assert.NilError(t, err)
	defer os.RemoveAll(tmp)

	char0 := filepath.Join(tmp, "char0")
	cmd := reexec.Command(reexecMknodChar0, char0)
	unshareCmd(cmd)
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, string(out))
	assert.NilError(t, isChar0(char0))

	opaqueDir := filepath.Join(tmp, "opaquedir")
	err = os.MkdirAll(opaqueDir, 0755)
	assert.NilError(t, err, string(out))
	cmd = reexec.Command(reexecSetOpaque, opaqueDir)
	unshareCmd(cmd)
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, string(out))
	assert.NilError(t, isOpaque(opaqueDir))
}

func init() {
	reexec.Register(reexecSupportsUserNSOverlay, func() {
		if err := supportsOverlay(os.Args[1]); err != nil {
			panic(err)
		}
	})
	reexec.Register(reexecMknodChar0, func() {
		if err := mknodChar0Overlay(os.Args[1]); err != nil {
			panic(err)
		}
	})
	reexec.Register(reexecSetOpaque, func() {
		if err := replaceDirWithOverlayOpaque(os.Args[1]); err != nil {
			panic(err)
		}
	})
	if reexec.Init() {
		os.Exit(0)
	}
}
