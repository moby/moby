// +build dfrunmount

package dockerfile2llb

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
)

func detectRunMount(cmd *command, allDispatchStates *dispatchStates) bool {
	if c, ok := cmd.Command.(*instructions.RunCommand); ok {
		mounts := instructions.GetMounts(c)
		sources := make([]*dispatchState, len(mounts))
		for i, mount := range mounts {
			if mount.From == "" && mount.Type == instructions.MountTypeCache {
				mount.From = emptyImageName
			}
			from := mount.From
			if from == "" || mount.Type == instructions.MountTypeTmpfs {
				continue
			}
			stn, ok := allDispatchStates.findStateByName(from)
			if !ok {
				stn = &dispatchState{
					stage:        instructions.Stage{BaseName: from},
					deps:         make(map[*dispatchState]struct{}),
					unregistered: true,
				}
			}
			sources[i] = stn
		}
		cmd.sources = sources
		return true
	}

	return false
}

func setCacheUIDGIDFileOp(m *instructions.Mount, st llb.State) llb.State {
	uid := 0
	gid := 0
	mode := os.FileMode(0755)
	if m.UID != nil {
		uid = int(*m.UID)
	}
	if m.GID != nil {
		gid = int(*m.GID)
	}
	if m.Mode != nil {
		mode = os.FileMode(*m.Mode)
	}
	return st.File(llb.Mkdir("/cache", mode, llb.WithUIDGID(uid, gid)), llb.WithCustomName("[internal] settings cache mount permissions"))
}

func setCacheUIDGID(m *instructions.Mount, st llb.State, fileop bool) llb.State {
	if fileop {
		return setCacheUIDGIDFileOp(m, st)
	}

	var b strings.Builder
	if m.UID != nil {
		b.WriteString(fmt.Sprintf("chown %d /mnt/cache;", *m.UID))
	}
	if m.GID != nil {
		b.WriteString(fmt.Sprintf("chown :%d /mnt/cache;", *m.GID))
	}
	if m.Mode != nil {
		b.WriteString(fmt.Sprintf("chmod %s /mnt/cache;", strconv.FormatUint(*m.Mode, 8)))
	}
	return llb.Image("busybox").Run(llb.Shlex(fmt.Sprintf("sh -c 'mkdir -p /mnt/cache;%s'", b.String())), llb.WithCustomName("[internal] settings cache mount permissions")).AddMount("/mnt", st)
}

func dispatchRunMounts(d *dispatchState, c *instructions.RunCommand, sources []*dispatchState, opt dispatchOpt) ([]llb.RunOption, error) {
	var out []llb.RunOption
	mounts := instructions.GetMounts(c)

	for i, mount := range mounts {
		if mount.From == "" && mount.Type == instructions.MountTypeCache {
			mount.From = emptyImageName
		}
		st := opt.buildContext
		if mount.From != "" {
			st = sources[i].state
		}
		var mountOpts []llb.MountOption
		if mount.Type == instructions.MountTypeTmpfs {
			st = llb.Scratch()
			mountOpts = append(mountOpts, llb.Tmpfs())
		}
		if mount.Type == instructions.MountTypeSecret {
			secret, err := dispatchSecret(mount)
			if err != nil {
				return nil, err
			}
			out = append(out, secret)
			continue
		}
		if mount.Type == instructions.MountTypeSSH {
			ssh, err := dispatchSSH(mount)
			if err != nil {
				return nil, err
			}
			out = append(out, ssh)
			continue
		}
		if mount.ReadOnly {
			mountOpts = append(mountOpts, llb.Readonly)
		} else if mount.Type == instructions.MountTypeBind && opt.llbCaps.Supports(pb.CapExecMountBindReadWriteNoOuput) == nil {
			mountOpts = append(mountOpts, llb.ForceNoOutput)
		}
		if mount.Type == instructions.MountTypeCache {
			sharing := llb.CacheMountShared
			if mount.CacheSharing == instructions.MountSharingPrivate {
				sharing = llb.CacheMountPrivate
			}
			if mount.CacheSharing == instructions.MountSharingLocked {
				sharing = llb.CacheMountLocked
			}
			if mount.CacheID == "" {
				mount.CacheID = path.Clean(mount.Target)
			}
			mountOpts = append(mountOpts, llb.AsPersistentCacheDir(opt.cacheIDNamespace+"/"+mount.CacheID, sharing))
		}
		target := mount.Target
		if !filepath.IsAbs(filepath.Clean(mount.Target)) {
			dir, err := d.state.GetDir(context.TODO())
			if err != nil {
				return nil, err
			}
			target = filepath.Join("/", dir, mount.Target)
		}
		if target == "/" {
			return nil, errors.Errorf("invalid mount target %q", target)
		}
		if src := path.Join("/", mount.Source); src != "/" {
			mountOpts = append(mountOpts, llb.SourcePath(src))
		} else {
			if mount.UID != nil || mount.GID != nil || mount.Mode != nil {
				st = setCacheUIDGID(mount, st, useFileOp(opt.buildArgValues, opt.llbCaps))
				mountOpts = append(mountOpts, llb.SourcePath("/cache"))
			}
		}

		out = append(out, llb.AddMount(target, st, mountOpts...))

		if mount.From == "" {
			d.ctxPaths[path.Join("/", filepath.ToSlash(mount.Source))] = struct{}{}
		}
	}
	return out, nil
}
