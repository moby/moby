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
		"helper": true, // mount helper, either built-in (by default) or external
	}
)

type optsConfig struct {
	MountType   string
	MountOpts   string
	MountDevice string
	MountHelper string
}

func (o *optsConfig) String() string {
	return fmt.Sprintf("type=%q device=%q o=%q helper=%q", o.MountType, o.MountDevice, o.MountOpts, o.MountHelper)
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
		MountHelper: opts["helper"],
	}
	return nil
}

func (v *localVolume) mount() error {
	if v.opts.MountDevice == "" {
		return fmt.Errorf("missing device in volume options")
	}
	mountOpts := v.opts.MountOpts
	// Supporting other local types other than nfs when helper=external
	if v.opts.MountHelper == "external" { // for helper=external
		// * Volume Example: docker volume create --driver local \
		//		--opt type=fuse.sshfs \
		//		--opt helper=external \
		//		--opt device=$(whoami)@localhost:/tmp \
		//		--opt o=workaround=all:o=reconnect:o=IdentityFile=/pub/user1.key:o=StrictHostKeyChecking=no \
		//		--name vol-name-1
		// * Swarm Example: docker service create image-test --mount type=volume,\
		//		volume-opt=type=fuse.sshfs,\
		//		volume-opt=helper=external,\
		//		volume-opt=device=$(whoami)@localhost:/tmp,\
		//		volume-opt=o=workaround=all:o=reconnect:o=IdentityFile=/pub/user1.key:o=StrictHostKeyChecking=no,\
		//		source=vol-name-1,target=/mount
		//
		// Examples will convert options into standard mount format:
		//		mount -t [opt:type] [opt:device] target -o [opt:o1] -o [opt:o2] ..
		//
		var args = []string{}
		if len(v.opts.MountType) > 0 {
			args = append(args, "-t", v.opts.MountType)
		}
		if len(mountOpts) > 0 {
			var opts = strings.Split(mountOpts, ":o=")
			for _, opt := range opts {
				args = append(args, "-o", opt)
			}
		}
		args = append(args, v.opts.MountDevice, v.path)
		if err := exec.Command("mount", args...).Run(); err != nil {
			v.Unmount(v.path)
			return fmt.Errorf("failed to mount by external helper: mount %s", strings.Join(args, " "))
		}
		return nil
	} else if v.opts.MountHelper != "" && v.opts.MountHelper != "built-in" { // for helper not in ('', 'built-in', 'external')
		return fmt.Errorf("unknown value for helper options: %s", v.opts.MountHelper)
	}
	// for helper=built-in
	if v.opts.MountType == "nfs" {
		if addrValue := getAddress(v.opts.MountOpts); addrValue != "" && net.ParseIP(addrValue).To4() == nil {
			ipAddr, err := net.ResolveIPAddr("ip", addrValue)
			if err != nil {
				return errors.Wrapf(err, "error resolving passed in nfs address")
			}
			mountOpts = strings.Replace(mountOpts, "addr="+addrValue, "addr="+ipAddr.String(), 1)
		}
	}
	err := mount.Mount(v.opts.MountDevice, v.path, v.opts.MountType, mountOpts)
	return errors.Wrapf(err, "error while mounting volume with options: %s", v.opts)
}
