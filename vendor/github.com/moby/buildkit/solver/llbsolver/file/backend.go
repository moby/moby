package file

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/llbsolver/ops/fileoptypes"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/sys/user"
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

func mkdir(d string, action *pb.FileActionMkDir, user *copy.User, idmap *user.IdentityMapping) (err error) {
	defer func() {
		var osErr *os.PathError
		if errors.As(err, &osErr) {
			osErr.Path = strings.TrimPrefix(osErr.Path, d)
		}
	}()

	p, err := fs.RootPath(d, action.Path)
	if err != nil {
		return errors.WithStack(err)
	}

	ch, err := mapUserToChowner(user, idmap)
	if err != nil {
		return err
	}

	if action.MakeParents {
		if _, err := copy.MkdirAll(p, os.FileMode(action.Mode)&0777, ch, timestampToTime(action.Timestamp)); err != nil {
			return err
		}
	} else {
		if err := os.Mkdir(p, os.FileMode(action.Mode)&0777); err != nil {
			if errors.Is(err, os.ErrExist) {
				return nil
			}
			return errors.WithStack(err)
		}
		if err := copy.Chown(p, nil, ch); err != nil {
			return errors.WithStack(err)
		}
		if err := copy.Utimes(p, timestampToTime(action.Timestamp)); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func symlink(d string, action *pb.FileActionSymlink, user *copy.User, idmap *user.IdentityMapping) (err error) {
	defer func() {
		var osErr *os.PathError
		if errors.As(err, &osErr) {
			// remove system root from error path if present
			osErr.Path = strings.TrimPrefix(osErr.Path, d)
		}
	}()

	newpath, err := fs.RootPath(d, filepath.Join("/", action.Newpath))
	if err != nil {
		return errors.WithStack(err)
	}

	ch, err := mapUserToChowner(user, idmap)
	if err != nil {
		return err
	}

	if err := os.Symlink(action.Oldpath, newpath); err != nil {
		return errors.WithStack(err)
	}

	if err := copy.Chown(newpath, nil, ch); err != nil {
		return errors.WithStack(err)
	}

	if err := copy.Utimes(newpath, timestampToTime(action.Timestamp)); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func mkfile(d string, action *pb.FileActionMkFile, user *copy.User, idmap *user.IdentityMapping) (err error) {
	defer func() {
		var osErr *os.PathError
		if errors.As(err, &osErr) {
			// remove system root from error path if present
			osErr.Path = strings.TrimPrefix(osErr.Path, d)
		}
	}()

	p, err := fs.RootPath(d, filepath.Join("/", action.Path))
	if err != nil {
		return errors.WithStack(err)
	}

	ch, err := mapUserToChowner(user, idmap)
	if err != nil {
		return err
	}

	if err := os.WriteFile(p, action.Data, os.FileMode(action.Mode)&0777); err != nil {
		return errors.WithStack(err)
	}

	if err := copy.Chown(p, nil, ch); err != nil {
		return errors.WithStack(err)
	}

	if err := copy.Utimes(p, timestampToTime(action.Timestamp)); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func rm(d string, action *pb.FileActionRm) (err error) {
	defer func() {
		var osErr *os.PathError
		if errors.As(err, &osErr) {
			// remove system root from error path if present
			osErr.Path = strings.TrimPrefix(osErr.Path, d)
		}
	}()

	if action.AllowWildcard {
		src := cleanPath(action.Path)
		m, err := copy.ResolveWildcards(d, src, false)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, s := range m {
			if err := rmPath(d, s, action.AllowNotFound); err != nil {
				return err
			}
		}

		return nil
	}

	return rmPath(d, action.Path, action.AllowNotFound)
}

func rmPath(root, src string, allowNotFound bool) error {
	src = filepath.Clean(src)
	dir, base := filepath.Split(src)
	if base == "" {
		return errors.New("rmPath: invalid empty path")
	}
	dir, err := fs.RootPath(root, filepath.Join("/", dir))
	if err != nil {
		return errors.WithStack(err)
	}
	p := filepath.Join(dir, base)

	if !allowNotFound {
		_, err := os.Stat(p)

		if errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}
	}

	return errors.WithStack(os.RemoveAll(p))
}

func docopy(ctx context.Context, src, dest string, action *pb.FileActionCopy, u *copy.User, idmap *user.IdentityMapping) (err error) {
	srcPath := cleanPath(action.Src)
	destPath := cleanPath(action.Dest)

	if !action.CreateDestPath {
		p, err := fs.RootPath(dest, filepath.Join("/", action.Dest))
		if err != nil {
			return errors.WithStack(err)
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
			ci.IncludePatterns = action.IncludePatterns
			ci.ExcludePatterns = action.ExcludePatterns
			ci.Chown = ch
			ci.Utime = timestampToTime(action.Timestamp)
			if action.ModeStr != "" {
				ci.ModeStr = action.ModeStr
			} else if m := int(action.Mode); m != -1 {
				ci.Mode = &m
			}
			ci.CopyDirContents = action.DirCopyContents
			ci.FollowLinks = action.FollowSymlink
			ci.AlwaysReplaceExistingDestPaths = action.AlwaysReplaceExistingDestPaths
		},
		copy.WithXAttrErrorHandler(xattrErrorHandler),
	}

	defer func() {
		var osErr *os.PathError
		if errors.As(err, &osErr) {
			// remove system root from error path if present
			osErr.Path = strings.TrimPrefix(osErr.Path, src)
			osErr.Path = strings.TrimPrefix(osErr.Path, dest)
		}
	}()

	var m []string
	if !action.AllowWildcard {
		m = []string{srcPath}
	} else {
		var err error
		m, err = copy.ResolveWildcards(src, srcPath, action.FollowSymlink)
		if err != nil {
			return errors.WithStack(err)
		}

		if len(m) == 0 {
			if action.AllowEmptyWildcard {
				return nil
			}
			return errors.Errorf("%s not found", srcPath)
		}
	}

	for _, s := range m {
		if action.AttemptUnpackDockerCompatibility {
			if ok, err := unpack(src, s, dest, destPath, ch, u, timestampToTime(action.Timestamp), idmap); err != nil {
				return errors.WithStack(err)
			} else if ok {
				continue
			}
		}
		if err := platformCopy(ctx, src, s, dest, destPath, opt...); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// NewFileOpBackend returns a new file operation backend. The executor is currently only used for Windows,
// and it is used to construct the readUserFn field set in the returned Backend.
func NewFileOpBackend(readUser ReadUserCallback) (*Backend, error) {
	if readUser == nil {
		return nil, errors.New("readUser callback must be provided")
	}
	return &Backend{
		readUser: readUser,
	}, nil
}

type ReadUserCallback func(chopt *pb.ChownOpt, mu, mg snapshot.Mountable) (*copy.User, error)

type Backend struct {
	readUser ReadUserCallback
}

func (fb *Backend) Mkdir(ctx context.Context, m, user, group fileoptypes.Mount, action *pb.FileActionMkDir) error {
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

	u, err := fb.readUserWrapper(action.Owner, user, group)
	if err != nil {
		return err
	}

	return mkdir(dir, action, u, mnt.m.IdentityMapping())
}

func (fb *Backend) Mkfile(ctx context.Context, m, user, group fileoptypes.Mount, action *pb.FileActionMkFile) error {
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

	u, err := fb.readUserWrapper(action.Owner, user, group)
	if err != nil {
		return err
	}

	return mkfile(dir, action, u, mnt.m.IdentityMapping())
}

func (fb *Backend) Symlink(ctx context.Context, m, user, group fileoptypes.Mount, action *pb.FileActionSymlink) error {
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

	u, err := fb.readUserWrapper(action.Owner, user, group)
	if err != nil {
		return err
	}

	return symlink(dir, action, u, mnt.m.IdentityMapping())
}

func (fb *Backend) Rm(ctx context.Context, m fileoptypes.Mount, action *pb.FileActionRm) error {
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

	return rm(dir, action)
}

func (fb *Backend) Copy(ctx context.Context, m1, m2, user, group fileoptypes.Mount, action *pb.FileActionCopy) error {
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

	u, err := fb.readUserWrapper(action.Owner, user, group)
	if err != nil {
		return err
	}

	return docopy(ctx, src, dest, action, u, mnt2.m.IdentityMapping())
}

func (fb *Backend) readUserWrapper(owner *pb.ChownOpt, user, group fileoptypes.Mount) (*copy.User, error) {
	var userMountable, groupMountable snapshot.Mountable
	if user != nil {
		usr, ok := user.(*Mount)
		if !ok {
			return nil, errors.Errorf("invalid mount type %T", user)
		}
		userMountable = usr.Mountable()
	}

	if group != nil {
		grp, ok := group.(*Mount)
		if !ok {
			return nil, errors.Errorf("invalid mount type %T", group)
		}
		groupMountable = grp.Mountable()
	}

	// We don't check the mountables for nil here. Depending on the ChownOpt value,
	// one of them may be nil. Allow the readUser function to handle this.
	u, err := fb.readUser(owner, userMountable, groupMountable)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func cleanPath(s string) string {
	s = filepath.FromSlash(s)
	s2 := filepath.Join("/", s)
	if strings.HasSuffix(s, string(filepath.Separator)+".") {
		if s2 != string(filepath.Separator) {
			s2 += string(filepath.Separator)
		}
		s2 += "."
	} else if strings.HasSuffix(s, string(filepath.Separator)) && s2 != string(filepath.Separator) {
		s2 += string(filepath.Separator)
	}
	return s2
}
