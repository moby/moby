package oci

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

func TestDeviceMode(t *testing.T) {
	tests := []struct {
		name string
		in   os.FileMode
		out  os.FileMode
	}{
		{name: "regular permissions", in: 0777, out: 0777},
		{name: "block device", in: 0777 | unix.S_IFBLK, out: 0777},
		{name: "character device", in: 0777 | unix.S_IFCHR, out: 0777},
		{name: "fifo device", in: 0777 | unix.S_IFIFO, out: 0777},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := Device(&configs.Device{FileMode: tc.in})
			assert.Equal(t, *d.FileMode, tc.out)
		})
	}
}
