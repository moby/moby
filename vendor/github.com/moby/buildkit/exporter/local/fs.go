package local

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/docker/docker/pkg/idtools"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/attestation"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/result"
	"github.com/moby/buildkit/util/staticfs"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	fstypes "github.com/tonistiigi/fsutil/types"
)

type CreateFSOpts struct {
	Epoch             *time.Time
	AttestationPrefix string
}

func CreateFS(ctx context.Context, sessionID string, k string, ref cache.ImmutableRef, attestations []exporter.Attestation, defaultTime time.Time, opt CreateFSOpts) (fsutil.FS, func() error, error) {
	var cleanup func() error
	var src string
	var err error
	var idmap *idtools.IdentityMapping
	if ref == nil {
		src, err = os.MkdirTemp("", "buildkit")
		if err != nil {
			return nil, nil, err
		}
		cleanup = func() error { return os.RemoveAll(src) }
	} else {
		mount, err := ref.Mount(ctx, true, session.NewGroup(sessionID))
		if err != nil {
			return nil, nil, err
		}

		lm := snapshot.LocalMounter(mount)

		src, err = lm.Mount()
		if err != nil {
			return nil, nil, err
		}

		idmap = mount.IdentityMapping()

		cleanup = lm.Unmount
	}

	walkOpt := &fsutil.WalkOpt{}
	var idMapFunc func(p string, st *fstypes.Stat) fsutil.MapResult

	if idmap != nil {
		idMapFunc = func(p string, st *fstypes.Stat) fsutil.MapResult {
			uid, gid, err := idmap.ToContainer(idtools.Identity{
				UID: int(st.Uid),
				GID: int(st.Gid),
			})
			if err != nil {
				return fsutil.MapResultExclude
			}
			st.Uid = uint32(uid)
			st.Gid = uint32(gid)
			return fsutil.MapResultKeep
		}
	}

	walkOpt.Map = func(p string, st *fstypes.Stat) fsutil.MapResult {
		res := fsutil.MapResultKeep
		if idMapFunc != nil {
			res = idMapFunc(p, st)
		}
		if opt.Epoch != nil {
			st.ModTime = opt.Epoch.UnixNano()
		}
		return res
	}

	outputFS := fsutil.NewFS(src, walkOpt)
	attestations = attestation.Filter(attestations, nil, map[string][]byte{
		result.AttestationInlineOnlyKey: []byte(strconv.FormatBool(true)),
	})
	attestations, err = attestation.Unbundle(ctx, session.NewGroup(sessionID), attestations)
	if err != nil {
		return nil, nil, err
	}
	if len(attestations) > 0 {
		subjects := []intoto.Subject{}
		err = outputFS.Walk(ctx, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			f, err := outputFS.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			d := digest.Canonical.Digester()
			if _, err := io.Copy(d.Hash(), f); err != nil {
				return err
			}
			subjects = append(subjects, intoto.Subject{
				Name:   path,
				Digest: result.ToDigestMap(d.Digest()),
			})
			return nil
		})
		if err != nil {
			return nil, nil, err
		}

		stmts, err := attestation.MakeInTotoStatements(ctx, session.NewGroup(sessionID), attestations, subjects)
		if err != nil {
			return nil, nil, err
		}
		stmtFS := staticfs.NewFS()

		names := map[string]struct{}{}
		for i, stmt := range stmts {
			dt, err := json.Marshal(stmt)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to marshal attestation")
			}

			name := opt.AttestationPrefix + path.Base(attestations[i].Path)
			if _, ok := names[name]; ok {
				return nil, nil, errors.Errorf("duplicate attestation path name %s", name)
			}
			names[name] = struct{}{}

			st := fstypes.Stat{
				Mode:    0600,
				Path:    name,
				ModTime: defaultTime.UnixNano(),
			}
			if opt.Epoch != nil {
				st.ModTime = opt.Epoch.UnixNano()
			}
			stmtFS.Add(name, st, dt)
		}

		outputFS = staticfs.NewMergeFS(outputFS, stmtFS)
	}

	return outputFS, cleanup, nil
}
