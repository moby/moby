package fstype

// FsMagic unsigned id of the filesystem in use.
type FsMagic uint32

const (
	FsMagicUnsupported FsMagic = 0x00000000 // FsMagicUnsupported is a predefined constant value other than a valid filesystem id.

	FsMagicAufs     FsMagic = 0x61756673 // FsMagicAufs filesystem id for Aufs.
	FsMagicBtrfs    FsMagic = 0x9123683E // FsMagicBtrfs filesystem id for Btrfs.
	FsMagicCramfs   FsMagic = 0x28cd3d45 // FsMagicCramfs filesystem id for Cramfs.
	FsMagicEcryptfs FsMagic = 0xf15f     // FsMagicEcryptfs filesystem id for eCryptfs.
	FsMagicExtfs    FsMagic = 0x0000EF53 // FsMagicExtfs filesystem id for Extfs.
	FsMagicF2fs     FsMagic = 0xF2F52010 // FsMagicF2fs filesystem id for F2fs.
	FsMagicGPFS     FsMagic = 0x47504653 // FsMagicGPFS filesystem id for GPFS.
	FsMagicJffs2Fs  FsMagic = 0x000072b6 // FsMagicJffs2Fs filesystem if for Jffs2Fs.
	FsMagicJfs      FsMagic = 0x3153464a // FsMagicJfs filesystem id for Jfs.
	FsMagicNfsFs    FsMagic = 0x00006969 // FsMagicNfsFs filesystem id for NfsFs.
	FsMagicRAMFs    FsMagic = 0x858458f6 // FsMagicRAMFs filesystem id for RamFs.
	FsMagicReiserFs FsMagic = 0x52654973 // FsMagicReiserFs filesystem id for ReiserFs.
	FsMagicSmbFs    FsMagic = 0x0000517B // FsMagicSmbFs filesystem id for SmbFs.
	FsMagicSquashFs FsMagic = 0x73717368 // FsMagicSquashFs filesystem id for SquashFs.
	FsMagicTmpFs    FsMagic = 0x01021994 // FsMagicTmpFs filesystem id for TmpFs.
	FsMagicVxFS     FsMagic = 0xa501fcf5 // FsMagicVxFS filesystem id for VxFs.
	FsMagicXfs      FsMagic = 0x58465342 // FsMagicXfs filesystem id for Xfs.
	FsMagicZfs      FsMagic = 0x2fc12fc1 // FsMagicZfs filesystem id for Zfs.
	FsMagicOverlay  FsMagic = 0x794C7630 // FsMagicOverlay filesystem id for overlayFs.
	FsMagicFUSE     FsMagic = 0x65735546 // FsMagicFUSE filesystem id for FUSE.
)

// FsNames maps filesystem id to name of the filesystem.
var FsNames = map[FsMagic]string{
	FsMagicUnsupported: "unsupported",

	FsMagicAufs:     "aufs",
	FsMagicBtrfs:    "btrfs",
	FsMagicCramfs:   "cramfs",
	FsMagicEcryptfs: "ecryptfs",
	FsMagicExtfs:    "extfs",
	FsMagicF2fs:     "f2fs",
	FsMagicFUSE:     "fuse",
	FsMagicGPFS:     "gpfs",
	FsMagicJffs2Fs:  "jffs2",
	FsMagicJfs:      "jfs",
	FsMagicNfsFs:    "nfs",
	FsMagicOverlay:  "overlayfs",
	FsMagicRAMFs:    "ramfs",
	FsMagicReiserFs: "reiserfs",
	FsMagicSmbFs:    "smb",
	FsMagicSquashFs: "squashfs",
	FsMagicTmpFs:    "tmpfs",
	FsMagicVxFS:     "vxfs",
	FsMagicXfs:      "xfs",
	FsMagicZfs:      "zfs",
}

// GetFSMagic returns the filesystem id given the path. It returns an error
// when failing to detect the filesystem. it returns [FsMagicUnsupported]
// if detection is not supported by the platform, but no error is returned
// in this case.
func GetFSMagic(rootpath string) (FsMagic, error) {
	return getFSMagic(rootpath)
}
