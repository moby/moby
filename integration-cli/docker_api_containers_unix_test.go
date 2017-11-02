// +build !windows

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/system"
	"github.com/go-check/check"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func (s *DockerSuite) TestContainersAPINetworkMountsNoChown(c *check.C) {
	// chown only applies to Linux bind mounted volumes; must be same host to verify
	testRequires(c, DaemonIsLinux, SameHostDaemon)

	tmpDir, err := ioutils.TempDir("", "test-network-mounts")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(tmpDir)

	// make tmp dir readable by anyone to allow userns process to mount from
	err = os.Chmod(tmpDir, 0755)
	c.Assert(err, checker.IsNil)
	// create temp files to use as network mounts
	tmpNWFileMount := filepath.Join(tmpDir, "nwfile")

	err = ioutil.WriteFile(tmpNWFileMount, []byte("network file bind mount"), 0644)
	c.Assert(err, checker.IsNil)

	config := containertypes.Config{
		Image: "busybox",
	}
	hostConfig := containertypes.HostConfig{
		Mounts: []mounttypes.Mount{
			{
				Type:   "bind",
				Source: tmpNWFileMount,
				Target: "/etc/resolv.conf",
			},
			{
				Type:   "bind",
				Source: tmpNWFileMount,
				Target: "/etc/hostname",
			},
			{
				Type:   "bind",
				Source: tmpNWFileMount,
				Target: "/etc/hosts",
			},
		},
	}

	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	ctrCreate, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, &networktypes.NetworkingConfig{}, "")
	c.Assert(err, checker.IsNil)
	// container will exit immediately because of no tty, but we only need the start sequence to test the condition
	err = cli.ContainerStart(context.Background(), ctrCreate.ID, types.ContainerStartOptions{})
	c.Assert(err, checker.IsNil)

	// check that host-located bind mount network file did not change ownership when the container was started
	statT, err := system.Stat(tmpNWFileMount)
	c.Assert(err, checker.IsNil)
	assert.Equal(c, uint32(0), statT.UID(), "bind mounted network file should not change ownership from root")
}
