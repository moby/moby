//go:build !windows

package mounts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/moby/buildkit/identity"
	"github.com/moby/sys/userns"
	"github.com/pkg/errors"
)

func (sm *secretMountInstance) Mount() ([]mount.Mount, func() error, error) {
	dir, err := os.MkdirTemp("", "buildkit-secrets")
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create temp dir")
	}
	cleanupDir := func() error {
		return os.RemoveAll(dir)
	}

	if err := os.Chmod(dir, 0711); err != nil {
		cleanupDir()
		return nil, nil, err
	}

	var mountOpts []string
	if sm.sm.mount.SecretOpt.Mode&0o111 == 0 {
		mountOpts = append(mountOpts, "noexec")
	}

	tmpMount := mount.Mount{
		Type:    "tmpfs",
		Source:  "tmpfs",
		Options: append([]string{"nodev", "nosuid", fmt.Sprintf("uid=%d,gid=%d", os.Geteuid(), os.Getegid())}, mountOpts...),
	}

	if userns.RunningInUserNS() {
		tmpMount.Options = nil
	}

	if err := mount.All([]mount.Mount{tmpMount}, dir); err != nil {
		cleanupDir()
		return nil, nil, errors.Wrap(err, "unable to setup secret mount")
	}
	sm.root = dir

	cleanup := func() error {
		if err := mount.Unmount(dir, 0); err != nil {
			return err
		}
		return cleanupDir()
	}

	randID := identity.NewID()
	fp := filepath.Join(dir, randID)
	if err := os.WriteFile(fp, sm.sm.data, 0600); err != nil {
		cleanup()
		return nil, nil, err
	}

	uid := int(sm.sm.mount.SecretOpt.Uid)
	gid := int(sm.sm.mount.SecretOpt.Gid)

	if sm.idmap != nil {
		uid, gid, err = sm.idmap.ToHost(uid, gid)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
	}

	if err := os.Chown(fp, uid, gid); err != nil {
		cleanup()
		return nil, nil, err
	}

	if err := os.Chmod(fp, os.FileMode(sm.sm.mount.SecretOpt.Mode&0777)); err != nil {
		cleanup()
		return nil, nil, err
	}

	return []mount.Mount{{
		Type:    "bind",
		Source:  fp,
		Options: append([]string{"ro", "rbind", "nodev", "nosuid"}, mountOpts...),
	}}, cleanup, nil
}
