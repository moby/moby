// +build linux

package devmapper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// FIXME: this is copy-pasted from the aufs driver.
// It should be moved into the core.

func Mounted(mountpoint string) (bool, error) {
	mntpoint, err := os.Stat(mountpoint)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	parent, err := os.Stat(filepath.Join(mountpoint, ".."))
	if err != nil {
		return false, err
	}
	mntpointSt := mntpoint.Sys().(*syscall.Stat_t)
	parentSt := parent.Sys().(*syscall.Stat_t)
	return mntpointSt.Dev != parentSt.Dev, nil
}

type probeData struct {
	fsName string
	magic  string
	offset uint64
}

func ProbeFsType(device string) (string, error) {
	probes := []probeData{
		{"btrfs", "_BHRfS_M", 0x10040},
		{"ext4", "\123\357", 0x438},
		{"xfs", "XFSB", 0},
	}

	maxLen := uint64(0)
	for _, p := range probes {
		l := p.offset + uint64(len(p.magic))
		if l > maxLen {
			maxLen = l
		}
	}

	file, err := os.Open(device)
	if err != nil {
		return "", err
	}

	buffer := make([]byte, maxLen)
	l, err := file.Read(buffer)
	if err != nil {
		return "", err
	}
	file.Close()
	if uint64(l) != maxLen {
		return "", fmt.Errorf("unable to detect filesystem type of %s, short read", device)
	}

	for _, p := range probes {
		if bytes.Equal([]byte(p.magic), buffer[p.offset:p.offset+uint64(len(p.magic))]) {
			return p.fsName, nil
		}
	}

	isZFS, err := probeZFS(device)
	if err != nil {
		return "", err
	}
	if isZFS {
		return "zfs", nil
	}

	return "", fmt.Errorf("Unknown filesystem type on %s", device)
}

const (
	zfsUberBlockBigEndian    = 0x00BAB10C
	zfsUberBlockLittleEndian = 0x0CB1BA00
)

// ZFS doesn't really have magic bytes at the start of the device like most
// other FSes do, but it does store an "Uberblock" quite early on the volume,
// which is guaranteed to start with the uint32 0x00BAB1OC ("oo-ba-bl-oc") in
// native endianness. Each Uberblock slot is 1024 bytes, and the first 4 bytes
// thereof are the magic constant. However, it's not totally predictable where
// we'll find this constant, so we scan the first 4 bytes of each of the first
// 2048 1024-byte aligned chunks (2MB).
//
// It's VERY likely that there's a smaller range we could scan and get the same
// results. This code plays it on the safe side.
func probeZFS(device string) (bool, error) {
	file, err := os.Open(device)
	if err != nil {
		return false, err
	}
	defer file.Close()

	blocksize := 1024
	count := 2048

	buffer := make([]byte, 4)
	for i := 0; i < count; i++ {
		offset := i * blocksize
		_, err := file.ReadAt(buffer, int64(offset))
		if err != nil {
			return false, err
		}
		oobabloc := binary.BigEndian.Uint32(buffer)
		if oobabloc == zfsUberBlockLittleEndian || oobabloc == zfsUberBlockBigEndian {
			return true, nil
		}
	}
	return false, nil
}

func joinMountOptions(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "," + b
}
