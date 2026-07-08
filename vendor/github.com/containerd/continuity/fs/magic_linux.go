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

/*
Copyright 2013-2018 Docker, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Original source: https://github.com/moby/moby/blob/v26.0.0/daemon/graphdriver/driver_linux.go

package fs

import (
	"path/filepath"
	"syscall"
)

// Magic unsigned id of the filesystem in use.
type Magic uint32

const (
	// MagicUnsupported is a predefined constant value other than a valid filesystem id.
	MagicUnsupported = Magic(0x00000000)
)

const (
	// MagicAufs filesystem id for Aufs
	MagicAufs = Magic(0x61756673)
	// MagicBtrfs filesystem id for Btrfs
	MagicBtrfs = Magic(0x9123683E)
	// MagicCramfs filesystem id for Cramfs
	MagicCramfs = Magic(0x28cd3d45)
	// MagicEcryptfs filesystem id for eCryptfs
	MagicEcryptfs = Magic(0xf15f)
	// MagicExtfs filesystem id for Extfs
	MagicExtfs = Magic(0x0000EF53)
	// MagicF2fs filesystem id for F2fs
	MagicF2fs = Magic(0xF2F52010)
	// MagicGPFS filesystem id for GPFS
	MagicGPFS = Magic(0x47504653)
	// MagicJffs2Fs filesystem if for Jffs2Fs
	MagicJffs2Fs = Magic(0x000072b6)
	// MagicJfs filesystem id for Jfs
	MagicJfs = Magic(0x3153464a)
	// MagicNfsFs filesystem id for NfsFs
	MagicNfsFs = Magic(0x00006969)
	// MagicRAMFs filesystem id for RamFs
	MagicRAMFs = Magic(0x858458f6)
	// MagicReiserFs filesystem id for ReiserFs
	MagicReiserFs = Magic(0x52654973)
	// MagicSmbFs filesystem id for SmbFs
	MagicSmbFs = Magic(0x0000517B)
	// MagicSquashFs filesystem id for SquashFs
	MagicSquashFs = Magic(0x73717368)
	// MagicTmpFs filesystem id for TmpFs
	MagicTmpFs = Magic(0x01021994)
	// MagicVxFS filesystem id for VxFs
	MagicVxFS = Magic(0xa501fcf5)
	// MagicXfs filesystem id for Xfs
	MagicXfs = Magic(0x58465342)
	// MagicZfs filesystem id for Zfs
	MagicZfs = Magic(0x2fc12fc1)
	// MagicOverlay filesystem id for overlay
	MagicOverlay = Magic(0x794C7630)
)

var (
	// FsNames maps filesystem id to name of the filesystem.
	FsNames = map[Magic]string{
		MagicAufs:        "aufs",
		MagicBtrfs:       "btrfs",
		MagicCramfs:      "cramfs",
		MagicExtfs:       "extfs",
		MagicF2fs:        "f2fs",
		MagicGPFS:        "gpfs",
		MagicJffs2Fs:     "jffs2",
		MagicJfs:         "jfs",
		MagicNfsFs:       "nfs",
		MagicOverlay:     "overlayfs",
		MagicRAMFs:       "ramfs",
		MagicReiserFs:    "reiserfs",
		MagicSmbFs:       "smb",
		MagicSquashFs:    "squashfs",
		MagicTmpFs:       "tmpfs",
		MagicUnsupported: "unsupported",
		MagicVxFS:        "vxfs",
		MagicXfs:         "xfs",
		MagicZfs:         "zfs",
	}
)

// GetMagic returns the filesystem id given the path.
func GetMagic(rootpath string) (Magic, error) {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(rootpath), &buf); err != nil {
		return 0, err
	}
	return Magic(buf.Type), nil
}
