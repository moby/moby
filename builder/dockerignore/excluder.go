package dockerignore

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// Excluder represents a set of patterns which should be excluded while walking
// a filesystem.
type Excluder struct {
	Excludes []Exclude

	// If specified not to be ".", specifies the directory where the
	// exclude rules are rooted.
	root string
}

// Exclude represents a single rule from a `.dockerignore`.
// Note that an exclude may be negated.
type Exclude struct {
	Pattern string
	Negated bool
	IsDir   bool
	glob.Glob
}

var errExcludeCommented = errors.New("exclude commented")

// NewExclude constructs an Exclude from a single line.
func NewExclude(line string) (Exclude, error) {
	exclude := Exclude{Pattern: line}
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return exclude, errExcludeCommented
	}

	exclude.Pattern = filepath.Clean(exclude.Pattern)

	if strings.HasPrefix(exclude.Pattern, "!") {
		exclude.Negated = true
		exclude.Pattern = exclude.Pattern[1:]
	}
	if strings.HasSuffix(exclude.Pattern, "/") {
		exclude.IsDir = true
		exclude.Pattern = trimTrailingSeps(exclude.Pattern)
	}
	var err error
	exclude.Glob, err = glob.Compile(exclude.Pattern, string([]rune{filepath.Separator}))
	return exclude, err
}

// NewExcluder loads the named ignoreFile path as an *Excluder.
// It trims whitespace and removes excludes prefixed with a "#" (comments).
func NewExcluder(root string, patterns []string) (*Excluder, error) {
	excluder := &Excluder{root: root}

	for _, pattern := range patterns {
		exclude, err := NewExclude(pattern)
		if err == errExcludeCommented {
			continue
		}
		if err != nil {
			return nil, err
		}
		excluder.Excludes = append(excluder.Excludes, exclude)
	}
	return excluder, nil
}

func trimTrailingSeps(filePath string) string {
	if filePath == string([]rune{filepath.Separator}) {
		// Path is "/", so don't trim it.
		return filePath
	}
	return strings.TrimFunc(filePath, func(r rune) bool {
		return r == filepath.Separator
	})
}

// LastMatch returns the last matching excluder (beware: it may be negated.)
func (e *Excluder) LastMatch(filePath string, isDir bool) (*Exclude, bool) {
	if isDir && strings.HasSuffix(filePath, string([]rune{filepath.Separator})) {
		filePath = trimTrailingSeps(filePath)
	}
	// Consider excludes in reverse, since "last pattern wins" in the case
	// of negation.
	for i := len(e.Excludes) - 1; 0 <= i; i-- {
		e := &e.Excludes[i]
		if e.IsDir && !isDir {
			// This exclude only applies to directories,
			// and filePath is not a directory.
			continue
		}
		if e.Match(filePath) {
			return e, true
		}
	}
	return nil, false
}

// HasAnyNegation returns true if any of the exclude rules are includes.
func (e *Excluder) HasAnyNegation() bool {
	for _, rule := range e.Excludes {
		if rule.Negated {
			return true
		}
	}
	return false
}

// Wrap encloses the specified WalkFunc, only passing files to it which do not
// match patterns which are excluded due to the Exclusion list.
//
// Note: This function is not optimal in the presence of negations.
//       In particular, if there is even a single negation present, we walk all
//       directories in the tree, in case a negation matches in a subtree.
//       Fortunately, walking in general is quite fast, so this should only
//       impact a small number of users. It may be worth considering optimizing
//       this case though.
func (e *Excluder) Wrap(wrapped filepath.WalkFunc) filepath.WalkFunc {

	excludedDirectories := map[string]struct{}{}

	// This is a safety net. We assert that the walk must show us the
	// directory itself before showing the contents.
	seenDirectories := map[string]struct{}{}

	var inExcludedSubdirectory func(filePath string) bool
	// Strip the baseName from filePath repeatedly until the bottom is
	// reached. Returns true if any of the dirNames consider are present in
	// excludedDirectories.
	inExcludedSubdirectory = func(filePath string) bool {
		dirName := trimTrailingSeps(filepath.Dir(filePath))
		if _, seenDirectory := seenDirectories[dirName]; !seenDirectory {
			// panic("walk function gave us invalid data")
			// test if dirName should be ignored
			e, match := e.LastMatch(dirName, true)
			if match && !e.Negated {
				log.Printf("Ignore %q", dirName)
				excludedDirectories[dirName] = struct{}{}
			}
			seenDirectories[dirName] = struct{}{}
		}
		_, excluded := excludedDirectories[dirName]
		if excluded {
			return true
		}
		if dirName == "." || dirName == "" || dirName == "/" {
			return false
		}
		return inExcludedSubdirectory(dirName)
	}

	anyNegations := e.HasAnyNegation()

	return func(filePath string, f os.FileInfo, err error) error {
		if err != nil {
			// Forward errors directly to the wrapped function.
			return wrapped(filePath, f, err)
		}

		// Path to find exclusion rules matching against.
		testPath := filePath
		if e.root != "." {
			// We are performing a walk at some path other than ".",
			// So it is necessary to test the paths as though
			// they are files inside the `e.root` directory.
			testPath, err = filepath.Rel(e.root, filePath)
		}

		if f.IsDir() {
			seenDirectories[filePath] = struct{}{}
		}

		exclude, match := e.LastMatch(testPath, f.IsDir())

		switch {
		case match && !exclude.Negated && !f.IsDir():
			// The path matches an exclude which is not negated
			// and is not a directory. We can drop it.
			return nil

		case match && !exclude.Negated && f.IsDir():
			// The path matches an exclude which is not negated
			// but it is a directory.

			if !anyNegations {
				// There are no negation rules to consider.
				// We can halt recursive descent.
				return filepath.SkipDir
			}

			// In this case, there are negations.
			// It's possible there are cases where a file in an
			// excluded directory to become re-included, so
			// recursive descent must continue.

			// NOTE(pwaller): This is not the most efficient thing
			//                we could do at this point, but it's
			//                good enough for a first pass.
			// (A better thing to do would be to compute the rules
			//  which could affect the directory being recursed
			//  into.)

			excludedDirectories[filePath] = struct{}{}

		case !match && inExcludedSubdirectory(filePath):
			// No exclusion rule specifically matches this path,
			// but it is in a subdirectory which matches.
			return nil

		case match && exclude.Negated:
			// Found a match, but it was an explicit include.
			// Pass it to wrapped.

		case !match:
			// No match found, pass it to wrapped.

		default:
			panic("unreachable")
		}

		return wrapped(filePath, f, err)
	}
}
