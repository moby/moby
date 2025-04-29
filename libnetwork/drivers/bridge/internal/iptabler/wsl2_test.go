//go:build linux

package iptabler

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMirroredWSL2Workaround(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		loopback0        bool
		userlandProxy    bool
		wslinfoPerm      os.FileMode // 0 for no-file
		expLoopback0Rule bool
	}{
		{
			desc: "No loopback0",
		},
		{
			desc:             "WSL2 mirrored",
			loopback0:        true,
			userlandProxy:    true,
			wslinfoPerm:      0o777,
			expLoopback0Rule: true,
		},
		{
			desc:          "loopback0 but wslinfo not executable",
			loopback0:     true,
			userlandProxy: true,
			wslinfoPerm:   0o666,
		},
		{
			desc:          "loopback0 but no wslinfo",
			loopback0:     true,
			userlandProxy: true,
		},
		{
			desc:        "loopback0 but no userland proxy",
			loopback0:   true,
			wslinfoPerm: 0o777,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			restoreWslinfoPath := simulateWSL2MirroredMode(t, tc.loopback0, tc.wslinfoPerm)
			defer restoreWslinfoPath()

			_, err := NewIptabler(context.Background(), firewaller.Config{
				IPv4:    true,
				Hairpin: !tc.userlandProxy,
			})
			assert.NilError(t, err)
			assert.Check(t, is.Equal(mirroredWSL2Rule().Exists(), tc.expLoopback0Rule))
		})
	}
}

// simulateWSL2MirroredMode simulates the WSL2 mirrored mode by creating a
// loopback0 interface and optionally creating a wslinfo file with the given
// permissions.
// A clean up function is returned and will restore the original wslinfoPath
// used within the 'bridge' package. The loopback0 interface isn't cleaned up.
// Instead this function should be called from a disposable network namespace.
func simulateWSL2MirroredMode(t *testing.T, loopback0 bool, wslinfoPerm os.FileMode) func() {
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

	return func() {
		wslinfoPath = wslinfoPathOrig
	}
}

func TestMirroredWSL2LoopbackFiltering(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		loopback0        bool
		wslinfoPerm      os.FileMode // 0 for no-file
		expLoopback0Rule bool
	}{
		{
			desc: "No loopback0",
		},
		{
			desc:             "WSL2 mirrored",
			loopback0:        true,
			wslinfoPerm:      0o777,
			expLoopback0Rule: true,
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
			restoreWslinfoPath := simulateWSL2MirroredMode(t, tc.loopback0, tc.wslinfoPerm)
			defer restoreWslinfoPath()

			hostIP := net.ParseIP("127.0.0.1")
			err := filterPortMappedOnLoopback(context.Background(), types.PortBinding{
				Proto:    types.TCP,
				IP:       hostIP,
				HostPort: 8000,
			}, hostIP, true)
			assert.NilError(t, err)

			out, err := exec.Command("iptables-save", "-t", "raw").CombinedOutput()
			assert.NilError(t, err)

			// Checking this after trying to create rules, to make sure the init code in iptables/firewalld.go has run.
			if fw, _ := iptables.UsingFirewalld(); fw {
				t.Skip("firewalld is running in the host netns, it can't modify rules in the test's netns")
			}

			if tc.expLoopback0Rule {
				assert.Check(t, is.Equal(strings.Count(string(out), "-A PREROUTING"), 2))
				assert.Check(t, is.Contains(string(out), "-A PREROUTING -d 127.0.0.1/32 -i loopback0 -p tcp -m tcp --dport 8000 -j ACCEPT"))
			} else {
				assert.Check(t, is.Equal(strings.Count(string(out), "-A PREROUTING"), 1))
				assert.Check(t, !strings.Contains(string(out), "loopback0"), "There should be no rule in the raw-PREROUTING chain")
			}
		})
	}
}
