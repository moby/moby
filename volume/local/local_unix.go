//go:build linux || freebsd

// Package local provides the default implementation for volumes. It
// is used to mount data volume containers and directories local to
// the host server.
package local // import "github.com/docker/docker/volume/local"

import (
	"fmt"
	"net"
	"net/url"
	"os"
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
	validOpts = map[string]struct{}{
		"type":   {}, // specify the filesystem type for mount, e.g. nfs
		"o":      {}, // generic mount options
		"device": {}, // device to mount from
		"size":   {}, // quota size limit
	}
	mandatoryOpts = map[string][]string{
		"device": {"type"},
		"type":   {"device"},
		"o":      {"device", "type"},
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

func (r *Root) validateOpts(opts map[string]string) error {
	if len(opts) == 0 {
		return nil
	}
	for opt := range opts {
		if _, ok := validOpts[opt]; !ok {
			return errdefs.InvalidParameter(errors.Errorf("invalid option: %q", opt))
		}
	}
	if typeOpt, deviceOpt := opts["type"], opts["device"]; typeOpt == "cifs" && deviceOpt != "" {
		deviceURL, err := url.Parse(deviceOpt)
		if err != nil {
			return errdefs.InvalidParameter(errors.Wrapf(err, "error parsing mount device url"))
		}
		if deviceURL.Port() != "" {
			return errdefs.InvalidParameter(errors.New("port not allowed in CIFS device URL, include 'port' in 'o='"))
		}
	}
	if val, ok := opts["size"]; ok {
		size, err := units.RAMInBytes(val)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		if size > 0 && r.quotaCtl == nil {
			return errdefs.InvalidParameter(errors.New("quota size requested but no quota support"))
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

func (v *localVolume) setOpts(opts map[string]string) error {
	if len(opts) == 0 {
		return nil
	}
	v.opts = &optsConfig{
		MountType:   opts["type"],
		MountOpts:   opts["o"],
		MountDevice: opts["device"],
	}
	if val, ok := opts["size"]; ok {
		size, err := units.RAMInBytes(val)
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
		if size > 0 && v.quotaCtl == nil {
			return errdefs.InvalidParameter(errors.New("quota size requested but no quota support"))
		}
		v.opts.Quota.Size = uint64(size)
	}
	return v.saveOpts()
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

func getMountOptions(opts *optsConfig, resolveIP func(string, string) (*net.IPAddr, error)) (mountDevice string, mountOpts string, _ error) {
	if opts.MountDevice == "" {
		return "", "", fmt.Errorf("missing device in volume options")
	}

	mountOpts = opts.MountOpts
	mountDevice = opts.MountDevice

	switch opts.MountType {
	case "nfs", "cifs":
		if addrValue := getAddress(opts.MountOpts); addrValue != "" && net.ParseIP(addrValue).To4() == nil {
			ipAddr, err := resolveIP("ip", addrValue)
			if err != nil {
				return "", "", errors.Wrap(err, "error resolving passed in network volume address")
			}
			mountOpts = strings.Replace(mountOpts, "addr="+addrValue, "addr="+ipAddr.String(), 1)
			break
		}

		if opts.MountType != "cifs" {
			break
		}

		deviceURL, err := url.Parse(mountDevice)
		if err != nil {
			return "", "", errors.Wrap(err, "error parsing mount device url")
		}
		if deviceURL.Host != "" && net.ParseIP(deviceURL.Host) == nil {
			ipAddr, err := resolveIP("ip", deviceURL.Host)
			if err != nil {
				return "", "", errors.Wrap(err, "error resolving passed in network volume address")
			}
			deviceURL.Host = ipAddr.String()
			dev, err := url.QueryUnescape(deviceURL.String())
			if err != nil {
				return "", "", fmt.Errorf("failed to unescape device URL: %q", deviceURL)
			}
			mountDevice = dev
		}
	}

	return mountDevice, mountOpts, nil
}

func (v *localVolume) mount() error {
	mountDevice, mountOpts, err := getMountOptions(v.opts, net.ResolveIPAddr)
	if err != nil {
		return err
	}

	if err := mount.Mount(mountDevice, v.path, v.opts.MountType, mountOpts); err != nil {
		if password := getPassword(v.opts.MountOpts); password != "" {
			err = errors.New(strings.Replace(err.Error(), "password="+password, "password=********", 1))
		}
		return errors.Wrap(err, "failed to mount local volume")
	}
	return nil
}

func (v *localVolume) postMount() error {
	if v.opts == nil {
		return nil
	}
	if v.opts.Quota.Size > 0 {
		if v.quotaCtl != nil {
			return v.quotaCtl.SetQuota(v.path, v.opts.Quota)
		} else {
			return errors.New("size quota requested for volume but no quota support")
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

// restoreIfMounted restores the mounted status if the _data directory is already mounted.
func (v *localVolume) restoreIfMounted() error {
	if v.needsMount() {
		// Check if the _data directory is already mounted.
		mounted, err := mountinfo.Mounted(v.path)
		if err != nil {
			return fmt.Errorf("failed to determine if volume _data path is already mounted: %w", err)
		}

		if mounted {
			// Mark volume as mounted, but don't increment active count. If
			// any container needs this, the refcount will be incremented
			// by the live-restore (if enabled).
			// In other case, refcount will be zero but the volume will
			// already be considered as mounted when Mount is called, and
			// only the refcount will be incremented.
			v.active.mounted = true
		}
	}

	return nil
}

func (v *localVolume) CreatedAt() (time.Time, error) {
	fileInfo, err := os.Stat(v.rootPath)
	if err != nil {
		return time.Time{}, err
	}
	sec, nsec := fileInfo.Sys().(*syscall.Stat_t).Ctim.Unix()
	return time.Unix(sec, nsec), nil
}
