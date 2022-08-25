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
	"github.com/tonistiigi/fsutil/types"
)

type WalkOpt struct {
	IncludePatterns []string
	ExcludePatterns []string
	// FollowPaths contains symlinks that are resolved into include patterns
	// before performing the fs walk
	FollowPaths []string
	Map         MapFunc
}

type MapFunc func(string, *types.Stat) MapResult

// The result of the walk function controls
// both how WalkDir continues and whether the path is kept.
type MapResult int

const (
	// Keep the current path and continue.
	MapResultKeep MapResult = iota

	// Exclude the current path and continue.
	MapResultExclude

	// Exclude the current path, and skip the rest of the dir.
	// If path is a dir, skip the current directory.
	// If path is a file, skip the rest of the parent directory.
	// (This matches the semantics of fs.SkipDir.)
	MapResultSkipDir
)

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

	var (
		includePatterns []string
		includeMatcher  *fileutils.PatternMatcher
		excludeMatcher  *fileutils.PatternMatcher
	)

	if opt != nil && opt.IncludePatterns != nil {
		includePatterns = make([]string, len(opt.IncludePatterns))
		copy(includePatterns, opt.IncludePatterns)
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

	patternChars := "*[]?^"
	if os.PathSeparator != '\\' {
		patternChars += `\`
	}

	onlyPrefixIncludes := true
	if len(includePatterns) != 0 {
		includeMatcher, err = fileutils.NewPatternMatcher(includePatterns)
		if err != nil {
			return errors.Wrapf(err, "invalid includepatterns: %s", opt.IncludePatterns)
		}

		for _, p := range includeMatcher.Patterns() {
			if !p.Exclusion() && strings.ContainsAny(patternWithoutTrailingGlob(p), patternChars) {
				onlyPrefixIncludes = false
				break
			}
		}

	}

	onlyPrefixExcludeExceptions := true
	if opt != nil && opt.ExcludePatterns != nil {
		excludeMatcher, err = fileutils.NewPatternMatcher(opt.ExcludePatterns)
		if err != nil {
			return errors.Wrapf(err, "invalid excludepatterns: %s", opt.ExcludePatterns)
		}

		for _, p := range excludeMatcher.Patterns() {
			if p.Exclusion() && strings.ContainsAny(patternWithoutTrailingGlob(p), patternChars) {
				onlyPrefixExcludeExceptions = false
				break
			}
		}
	}

	type visitedDir struct {
		fi               os.FileInfo
		path             string
		origpath         string
		pathWithSep      string
		includeMatchInfo fileutils.MatchInfo
		excludeMatchInfo fileutils.MatchInfo
		calledFn         bool
	}

	// used only for include/exclude handling
	var parentDirs []visitedDir

	seenFiles := make(map[uint64]string)
	return filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) (retErr error) {
		defer func() {
			if retErr != nil && isNotExist(retErr) {
				retErr = filepath.SkipDir
			}
		}()

		origpath := path
		path, err = filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Skip root
		if path == "." {
			return nil
		}

		var (
			dir   visitedDir
			isDir bool
		)
		if fi != nil {
			isDir = fi.IsDir()
		}

		if includeMatcher != nil || excludeMatcher != nil {
			for len(parentDirs) != 0 {
				lastParentDir := parentDirs[len(parentDirs)-1].pathWithSep
				if strings.HasPrefix(path, lastParentDir) {
					break
				}
				parentDirs = parentDirs[:len(parentDirs)-1]
			}

			if isDir {
				dir = visitedDir{
					fi:          fi,
					path:        path,
					origpath:    origpath,
					pathWithSep: path + string(filepath.Separator),
				}
			}
		}

		skip := false

		if includeMatcher != nil {
			var parentIncludeMatchInfo fileutils.MatchInfo
			if len(parentDirs) != 0 {
				parentIncludeMatchInfo = parentDirs[len(parentDirs)-1].includeMatchInfo
			}
			m, matchInfo, err := includeMatcher.MatchesUsingParentResults(path, parentIncludeMatchInfo)
			if err != nil {
				return errors.Wrap(err, "failed to match includepatterns")
			}

			if isDir {
				dir.includeMatchInfo = matchInfo
			}

			if !m {
				if isDir && onlyPrefixIncludes {
					// Optimization: we can skip walking this dir if no include
					// patterns could match anything inside it.
					dirSlash := path + string(filepath.Separator)
					for _, pat := range includeMatcher.Patterns() {
						if pat.Exclusion() {
							continue
						}
						patStr := patternWithoutTrailingGlob(pat) + string(filepath.Separator)
						if strings.HasPrefix(patStr, dirSlash) {
							goto passedIncludeFilter
						}
					}
					return filepath.SkipDir
				}
			passedIncludeFilter:
				skip = true
			}
		}

		if excludeMatcher != nil {
			var parentExcludeMatchInfo fileutils.MatchInfo
			if len(parentDirs) != 0 {
				parentExcludeMatchInfo = parentDirs[len(parentDirs)-1].excludeMatchInfo
			}
			m, matchInfo, err := excludeMatcher.MatchesUsingParentResults(path, parentExcludeMatchInfo)
			if err != nil {
				return errors.Wrap(err, "failed to match excludepatterns")
			}

			if isDir {
				dir.excludeMatchInfo = matchInfo
			}

			if m {
				if isDir && onlyPrefixExcludeExceptions {
					// Optimization: we can skip walking this dir if no
					// exceptions to exclude patterns could match anything
					// inside it.
					if !excludeMatcher.Exclusions() {
						return filepath.SkipDir
					}

					dirSlash := path + string(filepath.Separator)
					for _, pat := range excludeMatcher.Patterns() {
						if !pat.Exclusion() {
							continue
						}
						patStr := patternWithoutTrailingGlob(pat) + string(filepath.Separator)
						if strings.HasPrefix(patStr, dirSlash) {
							goto passedExcludeFilter
						}
					}
					return filepath.SkipDir
				}
			passedExcludeFilter:
				skip = true
			}
		}

		if walkErr != nil {
			if skip && errors.Is(walkErr, os.ErrPermission) {
				return nil
			}
			return walkErr
		}

		if includeMatcher != nil || excludeMatcher != nil {
			defer func() {
				if isDir {
					parentDirs = append(parentDirs, dir)
				}
			}()
		}

		if skip {
			return nil
		}

		dir.calledFn = true

		stat, err := mkstat(origpath, path, fi, seenFiles)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if opt != nil && opt.Map != nil {
				result := opt.Map(stat.Path, stat)
				if result == MapResultSkipDir {
					return filepath.SkipDir
				} else if result == MapResultExclude {
					return nil
				}
			}
			for i, parentDir := range parentDirs {
				if parentDir.calledFn {
					continue
				}
				parentStat, err := mkstat(parentDir.origpath, parentDir.path, parentDir.fi, seenFiles)
				if err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if opt != nil && opt.Map != nil {
					result := opt.Map(parentStat.Path, parentStat)
					if result == MapResultSkipDir || result == MapResultExclude {
						continue
					}
				}

				if err := fn(parentStat.Path, &StatInfo{parentStat}, nil); err != nil {
					return err
				}
				parentDirs[i].calledFn = true
			}
			if err := fn(stat.Path, &StatInfo{stat}, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func patternWithoutTrailingGlob(p *fileutils.Pattern) string {
	patStr := p.String()
	// We use filepath.Separator here because fileutils.Pattern patterns
	// get transformed to use the native path separator:
	// https://github.com/moby/moby/blob/79651b7a979b40e26af353ad283ca7ea5d67a855/pkg/fileutils/fileutils.go#L54
	patStr = strings.TrimSuffix(patStr, string(filepath.Separator)+"**")
	patStr = strings.TrimSuffix(patStr, string(filepath.Separator)+"*")
	return patStr
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
