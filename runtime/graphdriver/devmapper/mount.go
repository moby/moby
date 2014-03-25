// +build linux,amd64

package devmapper

import (
	"bytes"
	"fmt"
	"path/filepath"
)

// FIXME: this is copy-pasted from the aufs driver.
// It should be moved into the core.

var Mounted = func(mountpoint string) (bool, error) {
	mntpoint, err := osStat(mountpoint)
	if err != nil {
		if osIsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	parent, err := osStat(filepath.Join(mountpoint, ".."))
	if err != nil {
		return false, err
	}
	mntpointSt := toSysStatT(mntpoint.Sys())
	parentSt := toSysStatT(parent.Sys())
	return mntpointSt.Dev != parentSt.Dev, nil
}

type probeData struct {
	fsName string
	magic  string
	offset uint64
}

var ProbeFsType = func(device string) (string, error) {
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

	file, err := osOpen(device)
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

	return "", fmt.Errorf("Unknown filesystem type on %s", device)
}
