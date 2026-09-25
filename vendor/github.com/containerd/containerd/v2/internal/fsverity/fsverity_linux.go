/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package fsverity

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/containerd/containerd/v2/pkg/kernelversion"
	"golang.org/x/sys/unix"
)

type fsverityEnableArg struct {
	version       uint32
	hashAlgorithm uint32
	blockSize     uint32
	saltSize      uint32
	saltPtr       uint64
	sigSize       uint32
	reserved1     uint32
	sigPtr        uint64
	reserved2     [11]uint64
}

const (
	defaultBlockSize int    = 4096
	maxDigestSize    uint16 = 64
)

func IsSupported(rootPath string) (bool, error) {
	minKernelVersion := kernelversion.KernelVersion{Kernel: 5, Major: 4}
	s, err := kernelversion.GreaterEqualThan(minKernelVersion)
	if err != nil {
		return s, err
	}

	integrityDir, err := os.MkdirTemp(rootPath, ".fsverity-check-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(integrityDir)

	digestPath := filepath.Join(integrityDir, "supported")
	digestFile, err := os.Create(digestPath)
	if err != nil {
		return false, err
	}

	digestFile.Close()

	eerr := Enable(digestPath)
	if eerr != nil {
		return false, eerr
	}

	return true, nil
}

func IsEnabled(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	var attr int32

	_, _, flagErr := unix.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(unix.FS_IOC_GETFLAGS), uintptr(unsafe.Pointer(&attr)))
	if flagErr != 0 {
		return false, fmt.Errorf("error getting inode flags: %w", flagErr)
	}

	if attr&unix.FS_VERITY_FL == unix.FS_VERITY_FL {
		return true, nil
	}

	return false, nil
}

func Enable(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var args = &fsverityEnableArg{}
	args.version = 1
	args.hashAlgorithm = 1

	// fsverity block size should be the minimum between the page size
	// and the file system block size
	// If neither value is retrieved successfully, set fsverity block size to the default value
	blockSize := unix.Getpagesize()

	s := unix.Stat_t{}
	serr := unix.Stat(path, &s)
	if serr == nil && int(s.Blksize) < blockSize {
		blockSize = int(s.Blksize)
	}

	if blockSize <= 0 {
		blockSize = defaultBlockSize
	}

	args.blockSize = uint32(blockSize)

	_, _, errno := unix.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(unix.FS_IOC_ENABLE_VERITY), uintptr(unsafe.Pointer(args)))
	if errno != 0 {
		return fmt.Errorf("enable fsverity failed: %w", errno)
	}

	return nil
}
