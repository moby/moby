package fsutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/prefix"
	"github.com/tonistiigi/fsutil/types"
)

type WalkOpt struct {
	IncludePatterns []string
	ExcludePatterns []string
	// FollowPaths contains symlinks that are resolved into include patterns
	// before performing the fs walk
	FollowPaths []string
	Map         FilterFunc
}

func Walk(ctx context.Context, p string, opt *WalkOpt, fn filepath.WalkFunc) error {
	root, err := filepath.EvalSymlinks(p)
	if err != nil {
		return errors.WithStack(&os.PathError{Op: "resolve", Path: root, Err: err})
	}
	fi, err := os.Stat(root)
	if err != nil {
		return errors.WithStack(err)
	}
	if !fi.IsDir() {
		return errors.WithStack(&os.PathError{Op: "walk", Path: root, Err: syscall.ENOTDIR})
	}

	var pm *fileutils.PatternMatcher
	if opt != nil && opt.ExcludePatterns != nil {
		pm, err = fileutils.NewPatternMatcher(opt.ExcludePatterns)
		if err != nil {
			return errors.Wrapf(err, "invalid excludepatterns: %s", opt.ExcludePatterns)
		}
	}

	var includePatterns []string
	if opt != nil && opt.IncludePatterns != nil {
		includePatterns = make([]string, len(opt.IncludePatterns))
		for k := range opt.IncludePatterns {
			includePatterns[k] = filepath.Clean(opt.IncludePatterns[k])
		}
	}
	if opt != nil && opt.FollowPaths != nil {
		targets, err := FollowLinks(p, opt.FollowPaths)
		if err != nil {
			return err
		}
		if targets != nil {
			includePatterns = append(includePatterns, targets...)
			includePatterns = dedupePaths(includePatterns)
		}
	}

	var lastIncludedDir string

	seenFiles := make(map[uint64]string)
	return filepath.Walk(root, func(path string, fi os.FileInfo, err error) (retErr error) {
		defer func() {
			if retErr != nil && isNotExist(retErr) {
				retErr = filepath.SkipDir
			}
		}()
		if err != nil {
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
			if includePatterns != nil {
				skip := false
				if lastIncludedDir != "" {
					if strings.HasPrefix(path, lastIncludedDir+string(filepath.Separator)) {
						skip = true
					}
				}

				if !skip {
					matched := false
					partial := true
					for _, pattern := range includePatterns {
						if ok, p := prefix.Match(pattern, path, false); ok {
							matched = true
							if !p {
								partial = false
								break
							}
						}
					}
					if !matched {
						if fi.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if !partial && fi.IsDir() {
						lastIncludedDir = path
					}
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
		stat, err := mkstat(origpath, path, fi, seenFiles)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if opt != nil && opt.Map != nil {
				if allowed := opt.Map(stat.Path, stat); !allowed {
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
	*types.Stat
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

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
