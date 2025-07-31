//go:build linux

package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/moby/v2/internal/testutils/netnsutils"

	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMirroredWSL2Workaround(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		loopback0   bool
		wslinfoPerm os.FileMode // 0 for no-file
		expMirrored bool
	}{
		{
			desc: "No loopback0",
		},
		{
			desc:        "WSL2 mirrored",
			loopback0:   true,
			wslinfoPerm: 0o777,
			expMirrored: true,
		},
		{
			desc:        "loopback0 but wslinfo not executable",
			loopback0:   true,
			wslinfoPerm: 0o666,
		},
		{
			desc:      "loopback0 but no wslinfo",
			loopback0: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			simulateWSL2MirroredMode(t, tc.loopback0, tc.wslinfoPerm)
			assert.Check(t, is.Equal(isRunningUnderWSL2MirroredMode(context.Background()), tc.expMirrored))
		})
	}
}

// simulateWSL2MirroredMode simulates the WSL2 mirrored mode by creating a
// loopback0 interface and optionally creating a wslinfo file with the given
// permissions.
func simulateWSL2MirroredMode(t *testing.T, loopback0 bool, wslinfoPerm os.FileMode) {
	if loopback0 {
		iface := &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Name: "loopback0",
			},
		}
		err := netlink.LinkAdd(iface)
		assert.NilError(t, err)
	}

	wslinfoPathOrig := wslinfoPath
	if wslinfoPerm != 0 {
		tmpdir := t.TempDir()
		p := filepath.Join(tmpdir, "wslinfo")
		err := os.WriteFile(p, []byte("#!/bin/sh\necho dummy file\n"), wslinfoPerm)
		assert.NilError(t, err)
		wslinfoPath = p
	}

	t.Cleanup(func() {
		wslinfoPath = wslinfoPathOrig
	})
}
