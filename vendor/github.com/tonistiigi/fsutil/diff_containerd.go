package fsutil

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/tonistiigi/fsutil/types"
	"golang.org/x/sync/errgroup"
)

// Everything below is copied from containerd/fs. TODO: remove duplication @dmcgowan

// Const redefined because containerd/fs doesn't build on !linux

// ChangeKind is the type of modification that
// a change is making.
type ChangeKind int

const (
	// ChangeKindAdd represents an addition of
	// a file
	ChangeKindAdd ChangeKind = iota

	// ChangeKindModify represents a change to
	// an existing file
	ChangeKindModify

	// ChangeKindDelete represents a delete of
	// a file
	ChangeKindDelete
)

// ChangeFunc is the type of function called for each change
// computed during a directory changes calculation.
type ChangeFunc func(ChangeKind, string, os.FileInfo, error) error

const compareChunkSize = 32 * 1024

type currentPath struct {
	path string
	stat *types.Stat
	//	fullPath string
}

// doubleWalkDiff walks both directories to create a diff
func doubleWalkDiff(ctx context.Context, changeFn ChangeFunc, a, b walkerFn, filter FilterFunc, differ DiffType) (err error) {
	g, ctx := errgroup.WithContext(ctx)

	var (
		c1 = make(chan *currentPath, 128)
		c2 = make(chan *currentPath, 128)

		f1, f2 *currentPath
		rmdir  string
	)
	g.Go(func() error {
		defer close(c1)
		return a(ctx, c1)
	})
	g.Go(func() error {
		defer close(c2)
		return b(ctx, c2)
	})
	g.Go(func() error {
	loop0:
		for c1 != nil || c2 != nil {
			if f1 == nil && c1 != nil {
				f1, err = nextPath(ctx, c1)
				if err != nil {
					return err
				}
				if f1 == nil {
					c1 = nil
				}
			}

			if f2 == nil && c2 != nil {
				f2, err = nextPath(ctx, c2)
				if err != nil {
					return err
				}
				if f2 == nil {
					c2 = nil
				}
			}
			if f1 == nil && f2 == nil {
				continue
			}

			var f *types.Stat
			var f2copy *currentPath
			if f2 != nil {
				statCopy := *f2.stat
				if filter != nil {
					filter(f2.path, &statCopy)
				}
				f2copy = &currentPath{path: f2.path, stat: &statCopy}
			}
			k, p := pathChange(f1, f2copy)
			switch k {
			case ChangeKindAdd:
				if rmdir != "" {
					rmdir = ""
				}
				f = f2.stat
				f2 = nil
			case ChangeKindDelete:
				// Check if this file is already removed by being
				// under of a removed directory
				if rmdir != "" && strings.HasPrefix(f1.path, rmdir) {
					f1 = nil
					continue
				} else if rmdir == "" && f1.stat.IsDir() {
					rmdir = f1.path + string(os.PathSeparator)
				} else if rmdir != "" {
					rmdir = ""
				}
				f1 = nil
			case ChangeKindModify:
				same, err := sameFile(f1, f2copy, differ)
				if err != nil {
					return err
				}
				if f1.stat.IsDir() && !f2copy.stat.IsDir() {
					rmdir = f1.path + string(os.PathSeparator)
				} else if rmdir != "" {
					rmdir = ""
				}
				f = f2.stat
				f1 = nil
				f2 = nil
				if same {
					continue loop0
				}
			}
			if err := changeFn(k, p, &StatInfo{f}, nil); err != nil {
				return err
			}
		}
		return nil
	})

	return g.Wait()
}

func pathChange(lower, upper *currentPath) (ChangeKind, string) {
	if lower == nil {
		if upper == nil {
			panic("cannot compare nil paths")
		}
		return ChangeKindAdd, upper.path
	}
	if upper == nil {
		return ChangeKindDelete, lower.path
	}

	switch i := ComparePath(lower.path, upper.path); {
	case i < 0:
		// File in lower that is not in upper
		return ChangeKindDelete, lower.path
	case i > 0:
		// File in upper that is not in lower
		return ChangeKindAdd, upper.path
	default:
		return ChangeKindModify, upper.path
	}
}

func sameFile(f1, f2 *currentPath, differ DiffType) (same bool, retErr error) {
	if differ == DiffNone {
		return false, nil
	}
	// If not a directory also check size, modtime, and content
	if !f1.stat.IsDir() {
		if f1.stat.Size_ != f2.stat.Size_ {
			return false, nil
		}

		if f1.stat.ModTime != f2.stat.ModTime {
			return false, nil
		}
	}

	same, err := compareStat(f1.stat, f2.stat)
	if err != nil || !same || differ == DiffMetadata {
		return same, err
	}
	return compareFileContent(f1.path, f2.path)
}

func compareFileContent(p1, p2 string) (bool, error) {
	f1, err := os.Open(p1)
	if err != nil {
		return false, err
	}
	defer f1.Close()
	f2, err := os.Open(p2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	b1 := make([]byte, compareChunkSize)
	b2 := make([]byte, compareChunkSize)
	for {
		n1, err1 := f1.Read(b1)
		if err1 != nil && err1 != io.EOF {
			return false, err1
		}
		n2, err2 := f2.Read(b2)
		if err2 != nil && err2 != io.EOF {
			return false, err2
		}
		if n1 != n2 || !bytes.Equal(b1[:n1], b2[:n2]) {
			return false, nil
		}
		if err1 == io.EOF && err2 == io.EOF {
			return true, nil
		}
	}
}

// compareStat returns whether the stats are equivalent,
// whether the files are considered the same file, and
// an error
func compareStat(ls1, ls2 *types.Stat) (bool, error) {
	return ls1.Mode == ls2.Mode && ls1.Uid == ls2.Uid && ls1.Gid == ls2.Gid && ls1.Devmajor == ls2.Devmajor && ls1.Devminor == ls2.Devminor && ls1.Linkname == ls2.Linkname, nil
}

func nextPath(ctx context.Context, pathC <-chan *currentPath) (*currentPath, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p := <-pathC:
		return p, nil
	}
}
