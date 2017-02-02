package fsutil

import (
	"bytes"
	"io"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

// Everything below is copied from containerd/fs. TODO: remove duplication

// Const redefined because containerd/fs doesn't build on !linux

// ChangeKind is the type of modification that
// a change is making.
type ChangeKind int

const (
	// ChangeKindAdd represents an addition of
	// a file
	ChangeKindAdd = iota

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

type currentPath struct {
	path string
	f    os.FileInfo
	//	fullPath string
}

// doubleWalkDiff walks both directories to create a diff
func doubleWalkDiff(ctx context.Context, changeFn ChangeFunc, a, b walkerFn) (err error) {
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

			var f os.FileInfo
			k, p := pathChange(f1, f2)
			switch k {
			case ChangeKindAdd:
				if rmdir != "" {
					rmdir = ""
				}
				f = f2.f
				f2 = nil
			case ChangeKindDelete:
				// Check if this file is already removed by being
				// under of a removed directory
				if rmdir != "" && strings.HasPrefix(f1.path, rmdir) {
					f1 = nil
					continue
				} else if rmdir == "" && f1.f.IsDir() {
					rmdir = f1.path + string(os.PathSeparator)
				} else if rmdir != "" {
					rmdir = ""
				}
				f1 = nil
			case ChangeKindModify:
				same, err := sameFile(f1, f2)
				if err != nil {
					return err
				}
				if f1.f.IsDir() && !f2.f.IsDir() {
					rmdir = f1.path + string(os.PathSeparator)
				} else if rmdir != "" {
					rmdir = ""
				}
				f = f2.f
				f1 = nil
				f2 = nil
				if same {
					continue loop0
				}
			}
			if err := changeFn(k, p, f, nil); err != nil {
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
	// TODO: compare by directory

	switch i := strings.Compare(lower.path, upper.path); {
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

func sameFile(f1, f2 *currentPath) (bool, error) {
	// if os.SameFile(f1.f, f2.f) {
	//   return true, nil
	// }

	// todo: add these to os.FileInfo
	// equalStat, err := compareSysStat(f1.f.Sys(), f2.f.Sys())
	// if err != nil || !equalStat {
	//   return equalStat, err
	// }
	//
	// if eq, err := compareCapabilities(f1.fullPath, f2.fullPath); err != nil || !eq {
	//   return eq, err
	// }

	// If not a directory also check size, modtime, and content
	if !f1.f.IsDir() {
		if f1.f.Size() != f2.f.Size() {
			return false, nil
		}
		t1 := f1.f.ModTime()
		t2 := f2.f.ModTime()

		if t1.UnixNano() != t2.UnixNano() {
			return false, nil
		}

		// // If the timestamp may have been truncated in one of the
		// // files, check content of file to determine difference
		// if t1.Nanosecond() == 0 || t2.Nanosecond() == 0 {
		//   if f1.f.Size() > 0 {
		//     eq, err := compareFileContent(f1.fullPath, f2.fullPath)
		//     if err != nil || !eq {
		//       return eq, err
		//     }
		//   }
		// } else
		// if t1.Nanosecond() != t2.Nanosecond() {
		//       return false, nil
		//     }
	}
	return true, nil
}

const compareChuckSize = 32 * 1024

// compareFileContent compares the content of 2 same sized files
// by comparing each byte.
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

	b1 := make([]byte, compareChuckSize) // todo: pool
	b2 := make([]byte, compareChuckSize)
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

func nextPath(ctx context.Context, pathC <-chan *currentPath) (*currentPath, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p := <-pathC:
		return p, nil
	}
}
