package precompiledregexp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/scanner"

	"errors"
	"github.com/Sirupsen/logrus"
)

// PrecompiledRegExp represents a precompiled regular expression, and some
// pre-computed data for better performance. Meant for file pattern matching
type PrecompiledRegExp struct {
	pattern     string
	regExpr     *regexp.Regexp
	patternDirs []string
	negative    bool
}

// Pattern retrieves pattern, with ! in front if pattern is in the negative
func (regExpr PrecompiledRegExp) Pattern() string {
	if regExpr.negative {
		return "!" + regExpr.pattern
	}
	return regExpr.pattern
}

// RawPattern retrieves pattern, without !. Allows comparison of
// patterns between negative and non-negative regular expressions
func (regExpr PrecompiledRegExp) RawPattern() string {
	return regExpr.pattern
}

// RegExpr retrieves regular expression, should it ever be needed.
// Getter method, to prevent modification.
func (regExpr PrecompiledRegExp) RegExpr() *regexp.Regexp {
	return regExpr.regExpr
}

// Negative retrieves boolean flag as to whether Regular Expression is negative
func (regExpr PrecompiledRegExp) Negative() bool {
	return regExpr.negative
}

// PatternDirs retrieves directories that the pattern addresses.
func (regExpr PrecompiledRegExp) PatternDirs() []string {
	return regExpr.patternDirs
}

// Matches checks if this regular expression matches the string supplied. If pattern
// matches the argument, but regular expression is negative, will return false.
// The file provided should be provided as a path relative to the root context.
func (regExpr PrecompiledRegExp) Matches(relDir string) (bool, error) {
	matched := false

	match, err := regExpr.internalMatches(relDir)
	if err != nil {
		return false, err
	}

	if match {
		matched = !regExpr.negative
	}

	if matched {
		logrus.Debugf("Skipping excluded path: %s", relDir)
	}

	return matched, nil
}

// internalMatches checks if this regular expression matches the string supplied.
// Will not correct boolean return value of negative regular expressions.
// THe file provided should be provided as a path relative to the root context.
func (regExpr PrecompiledRegExp) internalMatches(relDir string) (bool, error) {
	if regExpr.regExpr == nil {
		return false, errors.New("Illegal exclusion pattern: " + regExpr.Pattern())
	}

	relDir = filepath.FromSlash(relDir)
	parentPathDirs := strings.Split(relDir, string(os.PathSeparator))

	match := regExpr.regExpr.MatchString(relDir)

	if !match && relDir != "." && relDir != "" {
		// Check to see if the pattern matches one of our parent dirs.
		if len(regExpr.patternDirs) < len(parentPathDirs) {
			match, _ = regExpr.internalMatches(strings.Join(parentPathDirs[:len(regExpr.patternDirs)], string(os.PathSeparator)))
		}
	}

	return match, nil
}

// NewPreCompiledRegExp will create a new instance of a PrecompiledRegExp,
// which will have the regular expression already generated. If an expression
// is expected to be used multiple times, use this to increase performance.
//
// NewPreCompiledRegExp tries to match the logic of filepath.Match but
// does so using regexp logic. We do this so that we can expand the
// wildcard set to include other things, like "**" to mean any number
// of directories.  This means that we should be backwards compatible
// with filepath.Match(). We'll end up supporting more stuff, due to
// the fact that we're using regexp, but that's ok - it does no harm.
//
// As per the comment in golangs filepath.Match, on Windows, escaping
// is disabled. Instead, '\\' is treated as path separator.
func NewPreCompiledRegExp(pattern string, isNegative bool) (*PrecompiledRegExp, error) {
	pattern = filepath.Clean(pattern)
	if pattern == "." {
		if isNegative {
			return &PrecompiledRegExp{
				pattern:     "",
				regExpr:     nil,
				negative:    true,
				patternDirs: nil,
			}, nil
		}
		return nil, errors.New("Invalid pattern; current directory")
	}

	regStr := "^"

	// Go through the pattern and convert it to a regexp.
	// We use a scanner so we can support utf-8 chars.
	var scan scanner.Scanner
	scan.Init(strings.NewReader(pattern))

	sl := string(os.PathSeparator)
	escSL := sl
	if sl == `\` {
		escSL += `\`
	}

	for scan.Peek() != scanner.EOF {
		ch := scan.Next()

		if ch == '*' {
			if scan.Peek() == '*' {
				// is some flavor of "**"
				scan.Next()

				if scan.Peek() == scanner.EOF {
					// is "**EOF" - to align with .gitignore just accept all
					regStr += ".*"
				} else {
					// is "**"
					regStr += "((.*" + escSL + ")|([^" + escSL + "]*))"
				}

				// Treat **/ as ** so eat the "/"
				if string(scan.Peek()) == sl {
					scan.Next()
				}
			} else {
				// is "*" so map it to anything but "/"
				regStr += "[^" + escSL + "]*"
			}
		} else if ch == '?' {
			// "?" is any char except "/"
			regStr += "[^" + escSL + "]"
		} else if strings.Index(".$", string(ch)) != -1 {
			if ch == '.' && scan.Peek() == '.' {
				scan.Next()
				if string(scan.Peek()) == sl {
					return nil, errors.New("Invalid pattern: cannot traverse up directory tree")
				}
				regStr += `\.\.`
			}
			// Escape some regexp special chars that have no meaning
			// in golang's filepath.Match
			regStr += `\` + string(ch)
		} else if ch == '\\' {
			// escape next char. Note that a trailing \ in the pattern
			// will be left alone (but need to escape it)
			if sl == `\` {
				// On windows map "\" to "\\", meaning an escaped backslash,
				// and then just continue because filepath.Match on
				// Windows doesn't allow escaping at all
				regStr += escSL
				continue
			}
			if scan.Peek() != scanner.EOF {
				regStr += `\` + string(scan.Next())
			} else {
				regStr += `\`
			}
		} else {
			regStr += string(ch)
		}
	}

	regStr += "$"

	regExpr, err := regexp.Compile(regStr)
	// Map regexp's error to filepath's so no one knows we're not using filepath
	if err != nil {
		err = filepath.ErrBadPattern
	}

	patternDirs := strings.Split(pattern, string(os.PathSeparator))

	return &PrecompiledRegExp{
		regExpr:     regExpr,
		pattern:     pattern,
		negative:    isNegative,
		patternDirs: patternDirs,
	}, err
}

// Matches returns true if file matches any of the regular expressions,
// and isn't excluded by any of the subsequent regular expressions.
func Matches(relPath string, regExprs []PrecompiledRegExp) (bool, error) {
	matches := false
	for _, entry := range regExprs {
		var err error
		matchesString, err := entry.internalMatches(relPath)
		if err != nil {
			return false, err
		}

		if matchesString {
			matches = !entry.Negative()
		}
	}

	return matches, nil
}

// ToStringExpressions converts the list of regular expressions to a string list,
// for logging or passing as command line arguments.
func ToStringExpressions(regExprs []PrecompiledRegExp) []string {
	stringExprs := make([]string, 0, len(regExprs))
	for _, entry := range regExprs {
		stringExprs = append(stringExprs, entry.Pattern())
	}
	return stringExprs
}

// FromStringExpressions creates a list of PrecompiledRegExps from a list of strings.
// This is used to compile a list of regular expressions from a list of the patterns.
func FromStringExpressions(stringExprs []string) ([]PrecompiledRegExp, error) {
	regExprs := make([]PrecompiledRegExp, 0, len(stringExprs))
	for _, pattern := range stringExprs {
		var regExpr *PrecompiledRegExp
		var err error

		if pattern == "" {
			return nil, errors.New("Illegal exclusion pattern: " + "\"\"")
		}

		isException := pattern[0] == '!'
		if isException {
			regExpr, err = NewPreCompiledRegExp(
				pattern[1:], true)
		} else {
			regExpr, err = NewPreCompiledRegExp(
				pattern, false)
		}
		if err != nil {
			return nil, err
		}

		regExprs = append(regExprs, *regExpr)
	}
	return regExprs, nil
}

// HasNegatives checks if any of the regular expressions in this list are negative.
// Allows some short-circuiting if we know none are negative.
func HasNegatives(regExprs []PrecompiledRegExp) bool {
	for _, entry := range regExprs {
		if entry.negative {
			return true
		}
	}
	return false
}
