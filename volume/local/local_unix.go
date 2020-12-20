// +build linux freebsd

// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local // import "github.com/docker/docker/volume/local"

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/quota"
	units "github.com/docker/go-units"
	"github.com/moby/sys/mount"
	"github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
)

var (
	oldVfsDir = filepath.Join("vfs", "dir")

	validOpts = map[string]struct{}{
		"type":   {}, // specify the filesystem type for mount, e.g. nfs
		"o":      {}, // generic mount options
		"device": {}, // device to mount from
		"size":   {}, // quota size limit
	}
	mandatoryOpts = map[string][]string{
		"device": []string{"type"},
		"type":   []string{"device"},
		"o":      []string{"device", "type"},
	}
)

type optsConfig struct {
	MountType   string
	MountOpts   string
	MountDevice string
	Quota       quota.Quota
}

func (o *optsConfig) String() string {
	return fmt.Sprintf("type='%s' device='%s' o='%s' size='%d'", o.MountType, o.MountDevice, o.MountOpts, o.Quota.Size)
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
	err := validateOpts(opts)
	if err != nil {
		return err
	}
	v.opts = &optsConfig{
		MountType:   opts["type"],
		MountOpts:   opts["o"],
		MountDevice: opts["device"],
	}
	if val, ok := opts["size"]; ok {
		size, err := units.RAMInBytes(val)
		if err != nil {
			return err
		}
		if size > 0 && v.quotaCtl == nil {
			return errdefs.InvalidParameter(errors.Errorf("quota size requested but no quota support"))
		}
		v.opts.Quota.Size = uint64(size)
	}
	return nil
}

func validateOpts(opts map[string]string) error {
	if len(opts) == 0 {
		return nil
	}
	for opt := range opts {
		if _, ok := validOpts[opt]; !ok {
			return errdefs.InvalidParameter(errors.Errorf("invalid option: %q", opt))
		}
	}
	for opt, reqopts := range mandatoryOpts {
		if _, ok := opts[opt]; ok {
			for _, reqopt := range reqopts {
				if _, ok := opts[reqopt]; !ok {
					return errdefs.InvalidParameter(errors.Errorf("missing required option: %q", reqopt))
				}
			}
		}
	}
	return nil
}

func unmount(path string) {
	_ = mount.Unmount(path)
}

func (v *localVolume) needsMount() bool {
	if v.opts == nil {
		return false
	}
	if v.opts.MountDevice != "" || v.opts.MountType != "" {
		return true
	}
	return false
}

func (v *localVolume) mount() error {
	if v.opts.MountDevice == "" {
		return fmt.Errorf("missing device in volume options")
	}
	mountOpts := v.opts.MountOpts
	switch v.opts.MountType {
	case "nfs", "cifs":
		if addrValue := getAddress(v.opts.MountOpts); addrValue != "" && net.ParseIP(addrValue).To4() == nil {
			ipAddr, err := net.ResolveIPAddr("ip", addrValue)
			if err != nil {
				return errors.Wrapf(err, "error resolving passed in network volume address")
			}
			mountOpts = strings.Replace(mountOpts, "addr="+addrValue, "addr="+ipAddr.String(), 1)
		}
	}
	err := mount.Mount(v.opts.MountDevice, v.path, v.opts.MountType, mountOpts)
	return errors.Wrap(err, "failed to mount local volume")
}

func (v *localVolume) postMount() error {
	if v.opts == nil {
		return nil
	}
	if v.opts.Quota.Size > 0 {
		if v.quotaCtl != nil {
			err := v.quotaCtl.SetQuota(v.path, v.opts.Quota)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("size quota requested for volume but no quota support")
		}
	}
	return nil
}

func (v *localVolume) unmount() error {
	if v.needsMount() {
		if err := mount.Unmount(v.path); err != nil {
			if mounted, mErr := mountinfo.Mounted(v.path); mounted || mErr != nil {
				return errdefs.System(err)
			}
		}
		v.active.mounted = false
	}
	return nil
}

func (v *localVolume) CreatedAt() (time.Time, error) {
	fileInfo, err := os.Stat(v.path)
	if err != nil {
		return time.Time{}, err
	}
	sec, nsec := fileInfo.Sys().(*syscall.Stat_t).Ctim.Unix()
	return time.Unix(sec, nsec), nil
}
