package fsutil

import (
	"context"
	"io"
	gofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/moby/patternmatcher"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

type FilterOpt struct {
	// IncludePatterns requires that the path matches at least one of the
	// specified patterns.
	IncludePatterns []string

	// ExcludePatterns requires that the path does not match any of the
	// specified patterns.
	ExcludePatterns []string

	// FollowPaths contains symlinks that are resolved into IncludePatterns
	// at the time of the call to NewFilterFS.
	FollowPaths []string

	// Map is called for each path that is included in the result.
	// The function can modify the stat info for each element, while the result
	// of the function controls both how Walk continues.
	Map MapFunc
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

type filterFS struct {
	fs FS

	includeMatcher              *patternmatcher.PatternMatcher
	excludeMatcher              *patternmatcher.PatternMatcher
	onlyPrefixIncludes          bool
	onlyPrefixExcludeExceptions bool

	mapFn MapFunc
}

// NewFilterFS creates a new FS that filters the given FS using the given
// FilterOpt.
//
// The returned FS will not contain any paths that do not match the provided
// include and exclude patterns, or that are are excluded using the mapping
// function.
//
// The FS is assumed to be a snapshot of the filesystem at the time of the
// call to NewFilterFS. If the underlying filesystem changes, calls to the
// underlying FS may be inconsistent.
func NewFilterFS(fs FS, opt *FilterOpt) (FS, error) {
	if opt == nil {
		return fs, nil
	}

	var includePatterns []string
	if opt.IncludePatterns != nil {
		includePatterns = make([]string, len(opt.IncludePatterns))
		copy(includePatterns, opt.IncludePatterns)
	}
	if opt.FollowPaths != nil {
		targets, err := FollowLinks(fs, opt.FollowPaths)
		if err != nil {
			return nil, err
		}
		if targets != nil {
			includePatterns = append(includePatterns, targets...)
			includePatterns = dedupePaths(includePatterns)
		}
	}

	patternChars := "*[]?^"
	if filepath.Separator != '\\' {
		patternChars += `\`
	}

	var (
		includeMatcher              *patternmatcher.PatternMatcher
		excludeMatcher              *patternmatcher.PatternMatcher
		err                         error
		onlyPrefixIncludes          = true
		onlyPrefixExcludeExceptions = true
	)

	if len(includePatterns) > 0 {
		includeMatcher, err = patternmatcher.New(includePatterns)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid includepatterns: %s", includePatterns)
		}

		for _, p := range includeMatcher.Patterns() {
			if !p.Exclusion() && strings.ContainsAny(patternWithoutTrailingGlob(p), patternChars) {
				onlyPrefixIncludes = false
				break
			}
		}

	}

	if len(opt.ExcludePatterns) > 0 {
		excludeMatcher, err = patternmatcher.New(opt.ExcludePatterns)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid excludepatterns: %s", opt.ExcludePatterns)
		}

		for _, p := range excludeMatcher.Patterns() {
			if p.Exclusion() && strings.ContainsAny(patternWithoutTrailingGlob(p), patternChars) {
				onlyPrefixExcludeExceptions = false
				break
			}
		}
	}

	return &filterFS{
		fs:                          fs,
		includeMatcher:              includeMatcher,
		excludeMatcher:              excludeMatcher,
		onlyPrefixIncludes:          onlyPrefixIncludes,
		onlyPrefixExcludeExceptions: onlyPrefixExcludeExceptions,
		mapFn:                       opt.Map,
	}, nil
}

func (fs *filterFS) Open(p string) (io.ReadCloser, error) {
	if fs.includeMatcher != nil {
		m, err := fs.includeMatcher.MatchesOrParentMatches(p)
		if err != nil {
			return nil, err
		}
		if !m {
			return nil, errors.Wrapf(os.ErrNotExist, "open %s", p)
		}
	}
	if fs.excludeMatcher != nil {
		m, err := fs.excludeMatcher.MatchesOrParentMatches(p)
		if err != nil {
			return nil, err
		}
		if m {
			return nil, errors.Wrapf(os.ErrNotExist, "open %s", p)
		}
	}
	return fs.fs.Open(p)
}

func (fs *filterFS) Walk(ctx context.Context, target string, fn gofs.WalkDirFunc) error {
	type visitedDir struct {
		entry            gofs.DirEntry
		pathWithSep      string
		includeMatchInfo patternmatcher.MatchInfo
		excludeMatchInfo patternmatcher.MatchInfo
		calledFn         bool
		skipFn           bool
	}

	// used only for include/exclude handling
	var parentDirs []visitedDir

	return fs.fs.Walk(ctx, target, func(path string, dirEntry gofs.DirEntry, walkErr error) (retErr error) {
		defer func() {
			if retErr != nil && isNotExist(retErr) {
				retErr = filepath.SkipDir
			}
		}()

		var (
			dir   visitedDir
			isDir bool
		)
		if dirEntry != nil {
			isDir = dirEntry.IsDir()
		}

		if fs.includeMatcher != nil || fs.excludeMatcher != nil {
			for len(parentDirs) != 0 {
				lastParentDir := parentDirs[len(parentDirs)-1].pathWithSep
				if strings.HasPrefix(path, lastParentDir) {
					break
				}
				parentDirs = parentDirs[:len(parentDirs)-1]
			}

			if isDir {
				dir = visitedDir{
					entry:       dirEntry,
					pathWithSep: path + string(filepath.Separator),
				}
			}
		}

		skip := false

		if fs.includeMatcher != nil {
			var parentIncludeMatchInfo patternmatcher.MatchInfo
			if len(parentDirs) != 0 {
				parentIncludeMatchInfo = parentDirs[len(parentDirs)-1].includeMatchInfo
			}
			m, matchInfo, err := fs.includeMatcher.MatchesUsingParentResults(path, parentIncludeMatchInfo)
			if err != nil {
				return errors.Wrap(err, "failed to match includepatterns")
			}

			if isDir {
				dir.includeMatchInfo = matchInfo
			}

			if !m {
				if isDir && fs.onlyPrefixIncludes {
					// Optimization: we can skip walking this dir if no include
					// patterns could match anything inside it.
					dirSlash := path + string(filepath.Separator)
					for _, pat := range fs.includeMatcher.Patterns() {
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

		if fs.excludeMatcher != nil {
			var parentExcludeMatchInfo patternmatcher.MatchInfo
			if len(parentDirs) != 0 {
				parentExcludeMatchInfo = parentDirs[len(parentDirs)-1].excludeMatchInfo
			}
			m, matchInfo, err := fs.excludeMatcher.MatchesUsingParentResults(path, parentExcludeMatchInfo)
			if err != nil {
				return errors.Wrap(err, "failed to match excludepatterns")
			}

			if isDir {
				dir.excludeMatchInfo = matchInfo
			}

			if m {
				if isDir && fs.onlyPrefixExcludeExceptions {
					// Optimization: we can skip walking this dir if no
					// exceptions to exclude patterns could match anything
					// inside it.
					if !fs.excludeMatcher.Exclusions() {
						return filepath.SkipDir
					}

					dirSlash := path + string(filepath.Separator)
					for _, pat := range fs.excludeMatcher.Patterns() {
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

		if fs.includeMatcher != nil || fs.excludeMatcher != nil {
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

		fi, err := dirEntry.Info()
		if err != nil {
			return err
		}
		stat, ok := fi.Sys().(*types.Stat)
		if !ok {
			return errors.WithStack(&os.PathError{Path: path, Err: syscall.EBADMSG, Op: "fileinfo without stat info"})
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if fs.mapFn != nil {
				result := fs.mapFn(stat.Path, stat)
				if result == MapResultSkipDir {
					return filepath.SkipDir
				} else if result == MapResultExclude {
					return nil
				}
			}
			for i, parentDir := range parentDirs {
				if parentDir.skipFn {
					return filepath.SkipDir
				}
				if parentDir.calledFn {
					continue
				}
				parentFi, err := parentDir.entry.Info()
				if err != nil {
					return err
				}
				parentStat, ok := parentFi.Sys().(*types.Stat)
				if !ok {
					return errors.WithStack(&os.PathError{Path: path, Err: syscall.EBADMSG, Op: "fileinfo without stat info"})
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if fs.mapFn != nil {
					result := fs.mapFn(parentStat.Path, parentStat)
					if result == MapResultExclude {
						continue
					} else if result == MapResultSkipDir {
						parentDirs[i].skipFn = true
						return filepath.SkipDir
					}
				}

				parentDirs[i].calledFn = true
				if err := fn(parentStat.Path, &DirEntryInfo{Stat: parentStat}, nil); err == filepath.SkipDir {
					parentDirs[i].skipFn = true
					return filepath.SkipDir
				} else if err != nil {
					return err
				}
			}
			if err := fn(stat.Path, &DirEntryInfo{Stat: stat}, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func Walk(ctx context.Context, p string, opt *FilterOpt, fn filepath.WalkFunc) error {
	f, err := NewFS(p)
	if err != nil {
		return err
	}
	f, err = NewFilterFS(f, opt)
	if err != nil {
		return err
	}
	return f.Walk(ctx, "/", func(path string, d gofs.DirEntry, err error) error {
		var info gofs.FileInfo
		if d != nil {
			var err2 error
			info, err2 = d.Info()
			if err == nil {
				err = err2
			}
		}
		return fn(path, info, err)
	})
}

func WalkDir(ctx context.Context, p string, opt *FilterOpt, fn gofs.WalkDirFunc) error {
	f, err := NewFS(p)
	if err != nil {
		return err
	}
	f, err = NewFilterFS(f, opt)
	if err != nil {
		return err
	}
	return f.Walk(ctx, "/", fn)
}

func patternWithoutTrailingGlob(p *patternmatcher.Pattern) string {
	patStr := p.String()
	// We use filepath.Separator here because patternmatcher.Pattern patterns
	// get transformed to use the native path separator:
	// https://github.com/moby/patternmatcher/blob/130b41bafc16209dc1b52a103fdac1decad04f1a/patternmatcher.go#L52
	patStr = strings.TrimSuffix(patStr, string(filepath.Separator)+"**")
	patStr = strings.TrimSuffix(patStr, string(filepath.Separator)+"*")
	return patStr
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
