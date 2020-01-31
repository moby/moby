package file

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/llbsolver/ops/fileoptypes"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
	copy "github.com/tonistiigi/fsutil/copy"
)

func timestampToTime(ts int64) *time.Time {
	if ts == -1 {
		return nil
	}
	tm := time.Unix(ts/1e9, ts%1e9)
	return &tm
}

func mapUserToChowner(user *copy.User, idmap *idtools.IdentityMapping) (copy.Chowner, error) {
	if user == nil {
		return func(old *copy.User) (*copy.User, error) {
			if old == nil {
				if idmap == nil {
					return nil, nil
				}
				old = &copy.User{} // root
			}
			if idmap != nil {
				identity, err := idmap.ToHost(idtools.Identity{
					UID: old.Uid,
					GID: old.Gid,
				})
				if err != nil {
					return nil, err
				}
				return &copy.User{Uid: identity.UID, Gid: identity.GID}, nil
			}
			return old, nil
		}, nil
	}
	u := *user
	if idmap != nil {
		identity, err := idmap.ToHost(idtools.Identity{
			UID: user.Uid,
			GID: user.Gid,
		})
		if err != nil {
			return nil, err
		}
		u.Uid = identity.UID
		u.Gid = identity.GID
	}
	return func(*copy.User) (*copy.User, error) {
		return &u, nil
	}, nil
}

func mkdir(ctx context.Context, d string, action pb.FileActionMkDir, user *copy.User, idmap *idtools.IdentityMapping) error {
	p, err := fs.RootPath(d, filepath.Join(filepath.Join("/", action.Path)))
	if err != nil {
		return err
	}

	ch, err := mapUserToChowner(user, idmap)
	if err != nil {
		return err
	}

	if action.MakeParents {
		if err := copy.MkdirAll(p, os.FileMode(action.Mode)&0777, ch, timestampToTime(action.Timestamp)); err != nil {
			return err
		}
	} else {
		if err := os.Mkdir(p, os.FileMode(action.Mode)&0777); err != nil {
			if os.IsExist(err) {
				return nil
			}
			return err
		}
		if err := copy.Chown(p, nil, ch); err != nil {
			return err
		}
		if err := copy.Utimes(p, timestampToTime(action.Timestamp)); err != nil {
			return err
		}
	}

	return nil
}

func mkfile(ctx context.Context, d string, action pb.FileActionMkFile, user *copy.User, idmap *idtools.IdentityMapping) error {
	p, err := fs.RootPath(d, filepath.Join(filepath.Join("/", action.Path)))
	if err != nil {
		return err
	}

	ch, err := mapUserToChowner(user, idmap)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(p, action.Data, os.FileMode(action.Mode)&0777); err != nil {
		return err
	}

	if err := copy.Chown(p, nil, ch); err != nil {
		return err
	}

	if err := copy.Utimes(p, timestampToTime(action.Timestamp)); err != nil {
		return err
	}

	return nil
}

func rm(ctx context.Context, d string, action pb.FileActionRm) error {
	p, err := fs.RootPath(d, filepath.Join(filepath.Join("/", action.Path)))
	if err != nil {
		return err
	}

	if err := os.RemoveAll(p); err != nil {
		if os.IsNotExist(errors.Cause(err)) && action.AllowNotFound {
			return nil
		}
		return err
	}

	return nil
}

func docopy(ctx context.Context, src, dest string, action pb.FileActionCopy, u *copy.User, idmap *idtools.IdentityMapping) error {
	srcPath := cleanPath(action.Src)
	destPath := cleanPath(action.Dest)

	if !action.CreateDestPath {
		p, err := fs.RootPath(dest, filepath.Join(filepath.Join("/", action.Dest)))
		if err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Dir(p)); err != nil {
			return errors.Wrapf(err, "failed to stat %s", action.Dest)
		}
	}

	xattrErrorHandler := func(dst, src, key string, err error) error {
		log.Println(err)
		return nil
	}

	ch, err := mapUserToChowner(u, idmap)
	if err != nil {
		return err
	}

	opt := []copy.Opt{
		func(ci *copy.CopyInfo) {
			ci.Chown = ch
			ci.Utime = timestampToTime(action.Timestamp)
			if m := int(action.Mode); m != -1 {
				ci.Mode = &m
			}
			ci.CopyDirContents = action.DirCopyContents
			ci.FollowLinks = action.FollowSymlink
		},
		copy.WithXAttrErrorHandler(xattrErrorHandler),
	}

	if !action.AllowWildcard {
		if action.AttemptUnpackDockerCompatibility {
			if ok, err := unpack(ctx, src, srcPath, dest, destPath, ch, timestampToTime(action.Timestamp)); err != nil {
				return err
			} else if ok {
				return nil
			}
		}
		return copy.Copy(ctx, src, srcPath, dest, destPath, opt...)
	}

	m, err := copy.ResolveWildcards(src, srcPath, action.FollowSymlink)
	if err != nil {
		return err
	}

	if len(m) == 0 {
		if action.AllowEmptyWildcard {
			return nil
		}
		return errors.Errorf("%s not found", srcPath)
	}

	for _, s := range m {
		if action.AttemptUnpackDockerCompatibility {
			if ok, err := unpack(ctx, src, s, dest, destPath, ch, timestampToTime(action.Timestamp)); err != nil {
				return err
			} else if ok {
				continue
			}
		}
		if err := copy.Copy(ctx, src, s, dest, destPath, opt...); err != nil {
			return err
		}
	}

	return nil
}

func cleanPath(s string) string {
	s2 := filepath.Join("/", s)
	if strings.HasSuffix(s, "/.") {
		if s2 != "/" {
			s2 += "/"
		}
		s2 += "."
	} else if strings.HasSuffix(s, "/") && s2 != "/" {
		s2 += "/"
	}
	return s2
}

type Backend struct {
}

func (fb *Backend) Mkdir(ctx context.Context, m, user, group fileoptypes.Mount, action pb.FileActionMkDir) error {
	mnt, ok := m.(*Mount)
	if !ok {
		return errors.Errorf("invalid mount type %T", m)
	}

	lm := snapshot.LocalMounter(mnt.m)
	dir, err := lm.Mount()
	if err != nil {
		return err
	}
	defer lm.Unmount()

	u, err := readUser(action.Owner, user, group)
	if err != nil {
		return err
	}

	return mkdir(ctx, dir, action, u, mnt.m.IdentityMapping())
}

func (fb *Backend) Mkfile(ctx context.Context, m, user, group fileoptypes.Mount, action pb.FileActionMkFile) error {
	mnt, ok := m.(*Mount)
	if !ok {
		return errors.Errorf("invalid mount type %T", m)
	}

	lm := snapshot.LocalMounter(mnt.m)
	dir, err := lm.Mount()
	if err != nil {
		return err
	}
	defer lm.Unmount()

	u, err := readUser(action.Owner, user, group)
	if err != nil {
		return err
	}

	return mkfile(ctx, dir, action, u, mnt.m.IdentityMapping())
}
func (fb *Backend) Rm(ctx context.Context, m fileoptypes.Mount, action pb.FileActionRm) error {
	mnt, ok := m.(*Mount)
	if !ok {
		return errors.Errorf("invalid mount type %T", m)
	}

	lm := snapshot.LocalMounter(mnt.m)
	dir, err := lm.Mount()
	if err != nil {
		return err
	}
	defer lm.Unmount()

	return rm(ctx, dir, action)
}
func (fb *Backend) Copy(ctx context.Context, m1, m2, user, group fileoptypes.Mount, action pb.FileActionCopy) error {
	mnt1, ok := m1.(*Mount)
	if !ok {
		return errors.Errorf("invalid mount type %T", m1)
	}
	mnt2, ok := m2.(*Mount)
	if !ok {
		return errors.Errorf("invalid mount type %T", m2)
	}

	lm := snapshot.LocalMounter(mnt1.m)
	src, err := lm.Mount()
	if err != nil {
		return err
	}
	defer lm.Unmount()

	lm2 := snapshot.LocalMounter(mnt2.m)
	dest, err := lm2.Mount()
	if err != nil {
		return err
	}
	defer lm2.Unmount()

	u, err := readUser(action.Owner, user, group)
	if err != nil {
		return err
	}

	return docopy(ctx, src, dest, action, u, mnt2.m.IdentityMapping())
}
