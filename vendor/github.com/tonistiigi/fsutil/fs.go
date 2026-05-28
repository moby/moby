package fsutil

import (
	"context"
	"io"
	gofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

type FS interface {
	Walk(context.Context, string, gofs.WalkDirFunc) error
	Open(string) (io.ReadCloser, error)
}

// NewFS creates a new FS from a root directory on the host filesystem.
func NewFS(root string) (FS, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, errors.WithStack(&os.PathError{Op: "resolve", Path: root, Err: err})
	}
	fi, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.WithStack(&os.PathError{Op: "stat", Path: root, Err: syscall.ENOTDIR})
	}

	return &fs{
		root: root,
	}, nil
}

type fs struct {
	root string
}

func (fs *fs) Walk(ctx context.Context, target string, fn gofs.WalkDirFunc) error {
	seenFiles := make(map[uint64]string)
	return filepath.WalkDir(filepath.Join(fs.root, target), func(path string, dirEntry gofs.DirEntry, walkErr error) (retErr error) {
		defer func() {
			if retErr != nil && isNotExist(retErr) {
				retErr = filepath.SkipDir
			}
		}()

		origpath := path
		path, err := filepath.Rel(fs.root, path)
		if err != nil {
			return err
		}
		// Skip root
		if path == "." {
			return nil
		}

		var entry gofs.DirEntry
		if dirEntry != nil {
			entry = &DirEntryInfo{
				path:      path,
				origpath:  origpath,
				entry:     dirEntry,
				seenFiles: seenFiles,
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn(path, entry, walkErr); err != nil {
				return err
			}
		}
		return nil
	})
}

func (fs *fs) Open(p string) (io.ReadCloser, error) {
	rc, err := os.Open(filepath.Join(fs.root, p))
	return rc, errors.WithStack(err)
}

type Dir struct {
	Stat *types.Stat
	FS   FS
}

func SubDirFS(dirs []Dir) (FS, error) {
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Stat.Path < dirs[j].Stat.Path
	})
	m := map[string]Dir{}
	for _, d := range dirs {
		if path.Base(d.Stat.Path) != d.Stat.Path {
			return nil, errors.WithStack(&os.PathError{Path: d.Stat.Path, Err: syscall.EISDIR, Op: "invalid path"})
		}
		if _, ok := m[d.Stat.Path]; ok {
			return nil, errors.WithStack(&os.PathError{Path: d.Stat.Path, Err: syscall.EEXIST, Op: "duplicate path"})
		}
		m[d.Stat.Path] = d
	}
	return &subDirFS{m: m, dirs: dirs}, nil
}

type subDirFS struct {
	m    map[string]Dir
	dirs []Dir
}

func (fs *subDirFS) Walk(ctx context.Context, target string, fn gofs.WalkDirFunc) error {
	first, rest, _ := strings.Cut(target, string(filepath.Separator))

	for _, d := range fs.dirs {
		if first != "" && first != d.Stat.Path {
			continue
		}

		fi := &StatInfo{d.Stat.Clone()}
		if !fi.IsDir() {
			return errors.WithStack(&os.PathError{Path: d.Stat.Path, Err: syscall.ENOTDIR, Op: "walk subdir"})
		}
		dStat := d.Stat.Clone()
		if err := fn(d.Stat.Path, &DirEntryInfo{Stat: dStat}, nil); err != nil {
			return err
		}
		if err := d.FS.Walk(ctx, rest, func(p string, entry gofs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			fi, err := entry.Info()
			if err != nil {
				return err
			}
			stat, ok := fi.Sys().(*types.Stat)
			if !ok {
				return errors.WithStack(&os.PathError{Path: d.Stat.Path, Err: syscall.EBADMSG, Op: "fileinfo without stat info"})
			}
			stat.Path = path.Join(d.Stat.Path, stat.Path)
			if stat.Linkname != "" {
				if fi.Mode()&os.ModeSymlink != 0 {
					if strings.HasPrefix(stat.Linkname, "/") {
						stat.Linkname = path.Join("/"+d.Stat.Path, stat.Linkname)
					}
				} else {
					stat.Linkname = path.Join(d.Stat.Path, stat.Linkname)
				}
			}
			return fn(filepath.Join(d.Stat.Path, p), &DirEntryInfo{Stat: stat}, nil)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (fs *subDirFS) Open(p string) (io.ReadCloser, error) {
	parts := strings.SplitN(filepath.Clean(p), string(filepath.Separator), 2)
	if len(parts) == 0 {
		return io.NopCloser(&emptyReader{}), nil
	}
	d, ok := fs.m[parts[0]]
	if !ok {
		return nil, errors.WithStack(&os.PathError{Path: parts[0], Err: syscall.ENOENT, Op: "open"})
	}
	return d.FS.Open(parts[1])
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

type StatInfo struct {
	*types.Stat
}

func (s *StatInfo) Name() string {
	return filepath.Base(s.Stat.Path)
}

func (s *StatInfo) Size() int64 {
	return s.Stat.Size
}

func (s *StatInfo) Mode() os.FileMode {
	return os.FileMode(s.Stat.Mode)
}

func (s *StatInfo) ModTime() time.Time {
	return time.Unix(s.Stat.ModTime/1e9, s.Stat.ModTime%1e9)
}

func (s *StatInfo) IsDir() bool {
	return s.Mode().IsDir()
}

func (s *StatInfo) Sys() interface{} {
	return s.Stat
}

type DirEntryInfo struct {
	*types.Stat

	entry     gofs.DirEntry
	path      string
	origpath  string
	seenFiles map[uint64]string
}

func (s *DirEntryInfo) Name() string {
	if s.Stat != nil {
		return filepath.Base(s.Stat.Path)
	}
	return s.entry.Name()
}

func (s *DirEntryInfo) IsDir() bool {
	if s.Stat != nil {
		return s.Stat.IsDir()
	}
	return s.entry.IsDir()
}

func (s *DirEntryInfo) Type() gofs.FileMode {
	if s.Stat != nil {
		return gofs.FileMode(s.Stat.Mode)
	}
	return s.entry.Type()
}

func (s *DirEntryInfo) Info() (gofs.FileInfo, error) {
	if s.Stat == nil {
		fi, err := s.entry.Info()
		if err != nil {
			return nil, err
		}
		stat, err := mkstat(s.origpath, s.path, fi, s.seenFiles)
		if err != nil {
			return nil, err
		}
		s.Stat = stat
	}

	st := s.Stat.Clone()
	return &StatInfo{st}, nil
}
