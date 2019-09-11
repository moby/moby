// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func openFUSEDevice() (*os.File, error) {
	fs, err := filepath.Glob("/dev/osxfuse*")
	if err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		bin := oldLoadBin
		if _, err := os.Stat(newLoadBin); err == nil {
			bin = newLoadBin
		}

		cmd := exec.Command(bin)
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		fs, err = filepath.Glob("/dev/osxfuse*")
		if err != nil {
			return nil, err
		}
	}

	for _, fn := range fs {
		f, err := os.OpenFile(fn, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		return f, nil
	}

	return nil, fmt.Errorf("all FUSE devices busy")
}

const oldLoadBin = "/Library/Filesystems/osxfusefs.fs/Support/load_osxfusefs"
const newLoadBin = "/Library/Filesystems/osxfuse.fs/Contents/Resources/load_osxfuse"

const oldMountBin = "/Library/Filesystems/osxfusefs.fs/Support/mount_osxfusefs"
const newMountBin = "/Library/Filesystems/osxfuse.fs/Contents/Resources/mount_osxfuse"

func mount(mountPoint string, opts *MountOptions, ready chan<- error) (fd int, err error) {
	f, err := openFUSEDevice()
	if err != nil {
		return 0, err
	}

	bin := oldMountBin
	if _, err := os.Stat(newMountBin); err == nil {
		bin = newMountBin
	}

	cmd := exec.Command(bin, "-o", strings.Join(opts.optionsStrings(), ","), "-o", fmt.Sprintf("iosize=%d", opts.MaxWrite), "3", mountPoint)
	cmd.ExtraFiles = []*os.File{f}
	cmd.Env = append(os.Environ(), "MOUNT_FUSEFS_CALL_BY_LIB=", "MOUNT_OSXFUSE_CALL_BY_LIB=",
		"MOUNT_OSXFUSE_DAEMON_PATH="+os.Args[0],
		"MOUNT_FUSEFS_DAEMON_PATH="+os.Args[0])

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Start(); err != nil {
		f.Close()
		return 0, err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			err = fmt.Errorf("mount_osxfusefs failed: %v. Stderr: %s, Stdout: %s", err, errOut.String(), out.String())
		}

		ready <- err
		close(ready)
	}()

	// The finalizer for f will close its fd so we return a dup.
	defer f.Close()
	return syscall.Dup(int(f.Fd()))
}

func unmount(dir string) error {
	return syscall.Unmount(dir, 0)
}
