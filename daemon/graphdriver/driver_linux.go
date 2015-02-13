// +build linux

package graphdriver

import (
	"path"
	"syscall"
)

const (
	FsMagicBtrfs    = FsMagic(0x9123683E)
	FsMagicAufs     = FsMagic(0x61756673)
	FsMagicExtfs    = FsMagic(0x0000EF53)
	FsMagicCramfs   = FsMagic(0x28cd3d45)
	FsMagicRamFs    = FsMagic(0x858458f6)
	FsMagicTmpFs    = FsMagic(0x01021994)
	FsMagicSquashFs = FsMagic(0x73717368)
	FsMagicNfsFs    = FsMagic(0x00006969)
	FsMagicReiserFs = FsMagic(0x52654973)
	FsMagicSmbFs    = FsMagic(0x0000517B)
	FsMagicJffs2Fs  = FsMagic(0x000072b6)
	FsMagicZfs      = FsMagic(0x2fc12fc1)
	FsMagicXfs      = FsMagic(0x58465342)
)

var (
	// Slice of drivers that should be used in an order
	priority = []string{
		"aufs",
		"btrfs",
		"devicemapper",
		"vfs",
		// experimental, has to be enabled manually for now
		"overlay",
	}

	FsNames = map[FsMagic]string{
		FsMagicAufs:        "aufs",
		FsMagicBtrfs:       "btrfs",
		FsMagicExtfs:       "extfs",
		FsMagicCramfs:      "cramfs",
		FsMagicRamFs:       "ramfs",
		FsMagicTmpFs:       "tmpfs",
		FsMagicSquashFs:    "squashfs",
		FsMagicNfsFs:       "nfs",
		FsMagicReiserFs:    "reiserfs",
		FsMagicSmbFs:       "smb",
		FsMagicJffs2Fs:     "jffs2",
		FsMagicZfs:         "zfs",
		FsMagicXfs:         "xfs",
		FsMagicUnsupported: "unsupported",
	}
)

func GetFSMagic(rootpath string) (FsMagic, error) {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path.Dir(rootpath), &buf); err != nil {
		return 0, err
	}
	return FsMagic(buf.Type), nil
}
