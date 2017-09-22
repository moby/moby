package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type WalkOpt struct {
	IncludePatterns []string
	ExcludePatterns []string
	Map             func(*Stat) bool
}

func Walk(ctx context.Context, p string, opt *WalkOpt, fn filepath.WalkFunc) error {
	root, err := filepath.EvalSymlinks(p)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve %s", root)
	}
	fi, err := os.Stat(root)
	if err != nil {
		return errors.Wrapf(err, "failed to stat: %s", root)
	}
	if !fi.IsDir() {
		return errors.Errorf("%s is not a directory", root)
	}

	var pm *fileutils.PatternMatcher
	if opt != nil && opt.ExcludePatterns != nil {
		pm, err = fileutils.NewPatternMatcher(opt.ExcludePatterns)
		if err != nil {
			return errors.Wrapf(err, "invalid excludepaths %s", opt.ExcludePatterns)
		}
	}

	seenFiles := make(map[uint64]string)
	return filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		origpath := path
		path, err = filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Skip root
		if path == "." {
			return nil
		}

		if opt != nil {
			if opt.IncludePatterns != nil {
				matched := false
				for _, p := range opt.IncludePatterns {
					if m, _ := filepath.Match(p, path); m {
						matched = true
						break
					}
				}
				if !matched {
					if fi.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if pm != nil {
				m, err := pm.Matches(path)
				if err != nil {
					return errors.Wrap(err, "failed to match excludepatterns")
				}

				if m {
					if fi.IsDir() {
						if !pm.Exclusions() {
							return filepath.SkipDir
						}
						dirSlash := path + string(filepath.Separator)
						for _, pat := range pm.Patterns() {
							if !pat.Exclusion() {
								continue
							}
							patStr := pat.String() + string(filepath.Separator)
							if strings.HasPrefix(patStr, dirSlash) {
								goto passedFilter
							}
						}
						return filepath.SkipDir
					}
					return nil
				}
			}
		}

	passedFilter:
		path = filepath.ToSlash(path)

		stat := &Stat{
			Path:    path,
			Mode:    uint32(fi.Mode()),
			Size_:   fi.Size(),
			ModTime: fi.ModTime().UnixNano(),
		}

		setUnixOpt(fi, stat, path, seenFiles)

		if !fi.IsDir() {
			if fi.Mode()&os.ModeSymlink != 0 {
				link, err := os.Readlink(origpath)
				if err != nil {
					return errors.Wrapf(err, "failed to readlink %s", origpath)
				}
				stat.Linkname = link
			}
		}
		if err := loadXattr(origpath, stat); err != nil {
			return errors.Wrapf(err, "failed to xattr %s", path)
		}

		if runtime.GOOS == "windows" {
			permPart := stat.Mode & uint32(os.ModePerm)
			noPermPart := stat.Mode &^ uint32(os.ModePerm)
			// Add the x bit: make everything +x from windows
			permPart |= 0111
			permPart &= 0755
			stat.Mode = noPermPart | permPart
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if opt != nil && opt.Map != nil {
				if allowed := opt.Map(stat); !allowed {
					return nil
				}
			}
			if err := fn(stat.Path, &StatInfo{stat}, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

type StatInfo struct {
	*Stat
}

func (s *StatInfo) Name() string {
	return filepath.Base(s.Stat.Path)
}
func (s *StatInfo) Size() int64 {
	return s.Stat.Size_
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
