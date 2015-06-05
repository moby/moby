package client

import (
	"github.com/docker/docker/api/types"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdMount mounts an image
//
// Usage: docker mount IMAGE DIR
func (cli *DockerCli) CmdMount(args ...string) error {
	cmd := cli.Subcmd("mount", "IMAGE DIR", "Mount image", true)
	cmd.Require(flag.Exact, 2)
	cmd.ParseFlags(args, true)
	var (
		image = cmd.Arg(0)
		dir   = cmd.Arg(1)
	)

	cfg := &types.MountConfig{
		MountDir: dir,
	}

	_, _, err := cli.call("POST", "/images/"+image+"/mount?dir="+dir, cfg, nil)
	if err != nil {
		return err
	}
	return nil
}

// CmdUmount unmounts an image
//
// Usage: docker umount IMAGE DIR
func (cli *DockerCli) CmdUmount(args ...string) error {
	var (
		cmd = cli.Subcmd("umount", "IMAGE DIR", "Unmount image", true)
	)
	cmd.Require(flag.Exact, 2)
	cmd.ParseFlags(args, true)
	var (
		image = cmd.Arg(0)
		dir   = cmd.Arg(1)
	)

	cfg := &types.MountConfig{
		MountDir: dir,
	}

	_, _, err := cli.call("POST", "/images/"+image+"/umount?dir="+dir, cfg, nil)
	if err != nil {
		return err
	}
	return nil
}
