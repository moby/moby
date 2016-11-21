// +build linux freebsd solaris

// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local

import (
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/docker/docker/pkg/mount"
)

var (
	oldVfsDir = filepath.Join("vfs", "dir")

	validOpts = map[string]bool{
		"type":   true, // specify the filesystem type for mount, e.g. nfs
		"o":      true, // generic mount options
		"device": true, // device to mount from
	}
)

type optsConfig struct {
	MountType   string
	MountOpts   string
	MountDevice string
}

func (o *optsConfig) String() string {
	return fmt.Sprintf("type='%s' device='%s' o='%s'", o.MountType, o.MountDevice, o.MountOpts)
}

// scopedPath verifies that the path where the volume is located
// is under Docker's root and the valid local paths.
func (r *Root) scopedPath(realPath string) bool {
	// Volumes path for Docker version >= 1.7
	if strings.HasPrefix(realPath, filepath.Join(r.scope, volumesPathName)) && realPath != filepath.Join(r.scope, volumesPathName) {
		return true
	}

	// Volumes path for Docker version < 1.7
	if strings.HasPrefix(realPath, filepath.Join(r.scope, oldVfsDir)) {
		return true
	}

	return false
}

func setOpts(v *localVolume, opts map[string]string) error {
	if len(opts) == 0 {
		return nil
	}
	if err := validateOpts(opts); err != nil {
		return err
	}

	v.opts = &optsConfig{
		MountType:   opts["type"],
		MountOpts:   opts["o"],
		MountDevice: opts["device"],
	}
	return nil
}

func (v *localVolume) mount() error {
	if v.opts.MountDevice == "" {
		return fmt.Errorf("missing device in volume options")
	}
	mountOpts := v.opts.MountOpts
	if v.opts.MountType == "nfs" {
		if addrValue := getAddress(v.opts.MountOpts); addrValue != "" && net.ParseIP(addrValue).To4() == nil {
			ipAddr, err := net.ResolveIPAddr("ip", addrValue)
			if err != nil {
				return errors.Wrapf(err, "error resolving passed in nfs address")
			}
			mountOpts = strings.Replace(mountOpts, "addr="+addrValue, "addr="+ipAddr.String(), 1)
		}
	}
	if strings.HasPrefix(v.opts.MountType, "^") {
		// Making docker local volume driver support external FS mount type, such as glusterfs, cephfs, fused-based fs (sshfs/curlftpfs/..) and so on. Options for each kind of fs-type are configurable via '--volume-opt'. To mount volume with external mount command, type option is supposed to start with '^'.
		// * SSHFS Example: docker service create .. --mount type=volume,volume-opt=type=^fuse.sshfs,volume-opt=device=ssh-node-1:/home/user1,volume-opt=o=workaround=all:o=reconnect:o=IdentityFile=/pub/user1.key:o=StrictHostKeyChecking=no,source=user1-home,target=/mount ..
		// * GlusterFS Example: docker service create .. --mount type=volume,volume-opt=type=^glusterfs,volume-opt=device=gfs-node-1:/shared,source=gfs-shared-1,target=/mount ..
		// * CephFS Example: docker service create .. --mount type=volume,volume-opt=type=^ceph,volume-opt=device=ceph-node-1:/shared,source=ceph-shared-1,target=/mount ..
		cmd := []string{"-t", v.opts.MountType[1:], v.opts.MountDevice, v.path}
		if v.opts.MountOpts != "" {
			opts := strings.Split(v.opts.MountOpts, ":o=")
			for i := 0; i < len(opts); i++ {
				cmd = append(cmd, []string{"-o", opts[i]}...)
			}
		}
		if out, err := exec.Command("mount", cmd...).Output(); err != nil {
			return errors.Wrapf(err, "error mounting volume using external mount for %s", out)
		}
		return nil
	}
	err := mount.Mount(v.opts.MountDevice, v.path, v.opts.MountType, mountOpts)
	return errors.Wrapf(err, "error while mounting volume with options: %s", v.opts)
}
