package fileutils

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWildcardMatches(t *testing.T) {
	match, _ := Matches("fileutils.go", []string{"*"})
	if !match {
		t.Errorf("failed to get a wildcard match, got %v", match)
	}
}

// A simple pattern match should return true.
func TestPatternMatches(t *testing.T) {
	match, _ := Matches("fileutils.go", []string{"*.go"})
	if !match {
		t.Errorf("failed to get a match, got %v", match)
	}
}

// An exclusion followed by an inclusion should return true.
func TestExclusionPatternMatchesPatternBefore(t *testing.T) {
	match, _ := Matches("fileutils.go", []string{"!fileutils.go", "*.go"})
	if !match {
		t.Errorf("failed to get true match on exclusion pattern, got %v", match)
	}
}

// A folder pattern followed by an exception should return false.
func TestPatternMatchesFolderExclusions(t *testing.T) {
	match, _ := Matches("docs/README.md", []string{"docs", "!docs/README.md"})
	if match {
		t.Errorf("failed to get a false match on exclusion pattern, got %v", match)
	}
}

// A folder pattern followed by an exception should return false.
func TestPatternMatchesFolderWithSlashExclusions(t *testing.T) {
	match, _ := Matches("docs/README.md", []string{"docs/", "!docs/README.md"})
	if match {
		t.Errorf("failed to get a false match on exclusion pattern, got %v", match)
	}
}

// A folder pattern followed by an exception should return false.
func TestPatternMatchesFolderWildcardExclusions(t *testing.T) {
	match, _ := Matches("docs/README.md", []string{"docs/*", "!docs/README.md"})
	if match {
		t.Errorf("failed to get a false match on exclusion pattern, got %v", match)
	}
}

// A pattern followed by an exclusion should return false.
func TestExclusionPatternMatchesPatternAfter(t *testing.T) {
	match, _ := Matches("fileutils.go", []string{"*.go", "!fileutils.go"})
	if match {
		t.Errorf("failed to get false match on exclusion pattern, got %v", match)
	}
}

// A filename evaluating to . should return false.
func TestExclusionPatternMatchesWholeDirectory(t *testing.T) {
	match, _ := Matches(".", []string{"*.go"})
	if match {
		t.Errorf("failed to get false match on ., got %v", match)
	}
}

// A single ! pattern should return an error.
func TestSingleExclamationError(t *testing.T) {
	_, err := Matches("fileutils.go", []string{"!"})
	if err == nil {
		t.Errorf("failed to get an error for a single exclamation point, got %v", err)
	}
}

// Matches with no patterns
func TestMatchesWithNoPatterns(t *testing.T) {
	matches, err := Matches("/any/path/there", []string{})
	if err != nil {
		t.Fatal(err)
	}
	if matches {
		t.Fatalf("Should not have match anything")
	}
}

// Matches with malformed patterns
func TestMatchesWithMalformedPatterns(t *testing.T) {
	matches, err := Matches("/any/path/there", []string{"["})
	if err == nil {
		t.Fatal("Should have failed because of a malformed syntax in the pattern")
	}
	if matches {
		t.Fatalf("Should not have match anything")
	}
}

type matchesTestCase struct {
	pattern string
	text    string
	pass    bool
}

type multiPatternTestCase struct {
	patterns []string
	text     string
	pass     bool
}

func TestMatches(t *testing.T) {
	tests := []matchesTestCase{
		{"**", "file", true},
		{"**", "file/", true},
		{"**/", "file", true}, // weird one
		{"**/", "file/", true},
		{"**", "/", true},
		{"**/", "/", true},
		{"**", "dir/file", true},
		{"**/", "dir/file", true},
		{"**", "dir/file/", true},
		{"**/", "dir/file/", true},
		{"**/**", "dir/file", true},
		{"**/**", "dir/file/", true},
		{"dir/**", "dir/file", true},
		{"dir/**", "dir/file/", true},
		{"dir/**", "dir/dir2/file", true},
		{"dir/**", "dir/dir2/file/", true},
		{"**/dir", "dir", true},
		{"**/dir", "dir/file", true},
		{"**/dir2/*", "dir/dir2/file", true},
		{"**/dir2/*", "dir/dir2/file/", true},
		{"**/dir2/**", "dir/dir2/dir3/file", true},
		{"**/dir2/**", "dir/dir2/dir3/file/", true},
		{"**file", "file", true},
		{"**file", "dir/file", true},
		{"**/file", "dir/file", true},
		{"**file", "dir/dir/file", true},
		{"**/file", "dir/dir/file", true},
		{"**/file*", "dir/dir/file", true},
		{"**/file*", "dir/dir/file.txt", true},
		{"**/file*txt", "dir/dir/file.txt", true},
		{"**/file*.txt", "dir/dir/file.txt", true},
		{"**/file*.txt*", "dir/dir/file.txt", true},
		{"**/**/*.txt", "dir/dir/file.txt", true},
		{"**/**/*.txt2", "dir/dir/file.txt", false},
		{"**/*.txt", "file.txt", true},
		{"**/**/*.txt", "file.txt", true},
		{"a**/*.txt", "a/file.txt", true},
		{"a**/*.txt", "a/dir/file.txt", true},
		{"a**/*.txt", "a/dir/dir/file.txt", true},
		{"a/*.txt", "a/dir/file.txt", false},
		{"a/*.txt", "a/file.txt", true},
		{"a/*.txt**", "a/file.txt", true},
		{"a[b-d]e", "ae", false},
		{"a[b-d]e", "ace", true},
		{"a[b-d]e", "aae", false},
		{"a[^b-d]e", "aze", true},
		{".*", ".foo", true},
		{".*", "foo", false},
		{"abc.def", "abcdef", false},
		{"abc.def", "abc.def", true},
		{"abc.def", "abcZdef", false},
		{"abc?def", "abcZdef", true},
		{"abc?def", "abcdef", false},
		{"a\\\\", "a\\", true},
		{"**/foo/bar", "foo/bar", true},
		{"**/foo/bar", "dir/foo/bar", true},
		{"**/foo/bar", "dir/dir2/foo/bar", true},
		{"abc/**", "abc", false},
		{"abc/**", "abc/def", true},
		{"abc/**", "abc/def/ghi", true},
		{"**/.foo", ".foo", true},
		{"**/.foo", "bar.foo", false},
		{"a(b)c/def", "a(b)c/def", true},
		{"a(b)c/def", "a(b)c/xyz", false},
		{"a.|)$(}+{bc", "a.|)$(}+{bc", true},
		{"dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl", "dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl", true},
		{"dist/*.whl", "dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl", true},
	}
	multiPatternTests := []multiPatternTestCase{
		{[]string{"**", "!util/docker/web"}, "util/docker/web/foo", false},
		{[]string{"**", "!util/docker/web", "util/docker/web/foo"}, "util/docker/web/foo", true},
		{[]string{"**", "!dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl"}, "dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl", false},
		{[]string{"**", "!dist/*.whl"}, "dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl", false},
	}

	if runtime.GOOS != "windows" {
		tests = append(tests, []matchesTestCase{
			{"a\\*b", "a*b", true},
		}...)
	}

	t.Run("MatchesOrParentMatches", func(t *testing.T) {
		for _, test := range tests {
			desc := fmt.Sprintf("pattern=%q text=%q", test.pattern, test.text)
			pm, err := NewPatternMatcher([]string{test.pattern})
			assert.NilError(t, err, desc)
			res, _ := pm.MatchesOrParentMatches(test.text)
			assert.Check(t, is.Equal(test.pass, res), desc)
		}

		for _, test := range multiPatternTests {
			desc := fmt.Sprintf("patterns=%q text=%q", test.patterns, test.text)
			pm, err := NewPatternMatcher(test.patterns)
			assert.NilError(t, err, desc)
			res, _ := pm.MatchesOrParentMatches(test.text)
			assert.Check(t, is.Equal(test.pass, res), desc)
		}
	})

	t.Run("MatchesUsingParentResult", func(t *testing.T) {
		for _, test := range tests {
			desc := fmt.Sprintf("pattern=%q text=%q", test.pattern, test.text)
			pm, err := NewPatternMatcher([]string{test.pattern})
			assert.NilError(t, err, desc)

			parentPath := filepath.Dir(filepath.FromSlash(test.text))
			parentPathDirs := strings.Split(parentPath, string(os.PathSeparator))

			parentMatched := false
			if parentPath != "." {
				for i := range parentPathDirs {
					parentMatched, _ = pm.MatchesUsingParentResult(strings.Join(parentPathDirs[:i+1], "/"), parentMatched)
				}
			}

			res, _ := pm.MatchesUsingParentResult(test.text, parentMatched)
			assert.Check(t, is.Equal(test.pass, res), desc)
		}
	})

	t.Run("MatchesUsingParentResults", func(t *testing.T) {
		check := func(pm *PatternMatcher, text string, pass bool, desc string) {
			parentPath := filepath.Dir(filepath.FromSlash(text))
			parentPathDirs := strings.Split(parentPath, string(os.PathSeparator))

			parentMatchInfo := MatchInfo{}
			if parentPath != "." {
				for i := range parentPathDirs {
					_, parentMatchInfo, _ = pm.MatchesUsingParentResults(strings.Join(parentPathDirs[:i+1], "/"), parentMatchInfo)
				}
			}

			res, _, _ := pm.MatchesUsingParentResults(text, parentMatchInfo)
			assert.Check(t, is.Equal(pass, res), desc)
		}

		for _, test := range tests {
			desc := fmt.Sprintf("pattern=%q text=%q", test.pattern, test.text)
			pm, err := NewPatternMatcher([]string{test.pattern})
			assert.NilError(t, err, desc)

			check(pm, test.text, test.pass, desc)
		}

		for _, test := range multiPatternTests {
			desc := fmt.Sprintf("pattern=%q text=%q", test.patterns, test.text)
			pm, err := NewPatternMatcher(test.patterns)
			assert.NilError(t, err, desc)

			check(pm, test.text, test.pass, desc)
		}
	})

	t.Run("MatchesUsingParentResultsNoContext", func(t *testing.T) {
		check := func(pm *PatternMatcher, text string, pass bool, desc string) {
			res, _, _ := pm.MatchesUsingParentResults(text, MatchInfo{})
			assert.Check(t, is.Equal(pass, res), desc)
		}

		for _, test := range tests {
			desc := fmt.Sprintf("pattern=%q text=%q", test.pattern, test.text)
			pm, err := NewPatternMatcher([]string{test.pattern})
			assert.NilError(t, err, desc)

			check(pm, test.text, test.pass, desc)
		}

		for _, test := range multiPatternTests {
			desc := fmt.Sprintf("pattern=%q text=%q", test.patterns, test.text)
			pm, err := NewPatternMatcher(test.patterns)
			assert.NilError(t, err, desc)

			check(pm, test.text, test.pass, desc)
		}
	})

}

func TestCleanPatterns(t *testing.T) {
	patterns := []string{"docs", "config"}
	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		t.Fatalf("invalid pattern %v", patterns)
	}
	cleaned := pm.Patterns()
	if len(cleaned) != 2 {
		t.Errorf("expected 2 element slice, got %v", len(cleaned))
	}
}

func TestCleanPatternsStripEmptyPatterns(t *testing.T) {
	patterns := []string{"docs", "config", ""}
	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		t.Fatalf("invalid pattern %v", patterns)
	}
	cleaned := pm.Patterns()
	if len(cleaned) != 2 {
		t.Errorf("expected 2 element slice, got %v", len(cleaned))
	}
}

func TestCleanPatternsExceptionFlag(t *testing.T) {
	patterns := []string{"docs", "!docs/README.md"}
	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		t.Fatalf("invalid pattern %v", patterns)
	}
	if !pm.Exclusions() {
		t.Errorf("expected exceptions to be true, got %v", pm.Exclusions())
	}
}

func TestCleanPatternsLeadingSpaceTrimmed(t *testing.T) {
	patterns := []string{"docs", "  !docs/README.md"}
	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		t.Fatalf("invalid pattern %v", patterns)
	}
	if !pm.Exclusions() {
		t.Errorf("expected exceptions to be true, got %v", pm.Exclusions())
	}
}

func TestCleanPatternsTrailingSpaceTrimmed(t *testing.T) {
	patterns := []string{"docs", "!docs/README.md  "}
	pm, err := NewPatternMatcher(patterns)
	if err != nil {
		t.Fatalf("invalid pattern %v", patterns)
	}
	if !pm.Exclusions() {
		t.Errorf("expected exceptions to be true, got %v", pm.Exclusions())
	}
}

func TestCleanPatternsErrorSingleException(t *testing.T) {
	patterns := []string{"!"}
	_, err := NewPatternMatcher(patterns)
	if err == nil {
		t.Errorf("expected error on single exclamation point, got %v", err)
	}
}

// These matchTests are stolen from go's filepath Match tests.
type matchTest struct {
	pattern, s string
	match      bool
	err        error
}

var matchTests = []matchTest{
	{"abc", "abc", true, nil},
	{"*", "abc", true, nil},
	{"*c", "abc", true, nil},
	{"a*", "a", true, nil},
	{"a*", "abc", true, nil},
	{"a*", "ab/c", true, nil},
	{"a*/b", "abc/b", true, nil},
	{"a*/b", "a/c/b", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/xxx/f", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/fff", false, nil},
	{"a*b?c*x", "abxbbxdbxebxczzx", true, nil},
	{"a*b?c*x", "abxbbxdbxebxczzy", false, nil},
	{"ab[c]", "abc", true, nil},
	{"ab[b-d]", "abc", true, nil},
	{"ab[e-g]", "abc", false, nil},
	{"ab[^c]", "abc", false, nil},
	{"ab[^b-d]", "abc", false, nil},
	{"ab[^e-g]", "abc", true, nil},
	{"a\\*b", "a*b", true, nil},
	{"a\\*b", "ab", false, nil},
	{"a?b", "a☺b", true, nil},
	{"a[^a]b", "a☺b", true, nil},
	{"a???b", "a☺b", false, nil},
	{"a[^a][^a][^a]b", "a☺b", false, nil},
	{"[a-ζ]*", "α", true, nil},
	{"*[a-ζ]", "A", false, nil},
	{"a?b", "a/b", false, nil},
	{"a*b", "a/b", false, nil},
	{"[\\]a]", "]", true, nil},
	{"[\\-]", "-", true, nil},
	{"[x\\-]", "x", true, nil},
	{"[x\\-]", "-", true, nil},
	{"[x\\-]", "z", false, nil},
	{"[\\-x]", "x", true, nil},
	{"[\\-x]", "-", true, nil},
	{"[\\-x]", "a", false, nil},
	{"[]a]", "]", false, filepath.ErrBadPattern},
	{"[-]", "-", false, filepath.ErrBadPattern},
	{"[x-]", "x", false, filepath.ErrBadPattern},
	{"[x-]", "-", false, filepath.ErrBadPattern},
	{"[x-]", "z", false, filepath.ErrBadPattern},
	{"[-x]", "x", false, filepath.ErrBadPattern},
	{"[-x]", "-", false, filepath.ErrBadPattern},
	{"[-x]", "a", false, filepath.ErrBadPattern},
	{"\\", "a", false, filepath.ErrBadPattern},
	{"[a-b-c]", "a", false, filepath.ErrBadPattern},
	{"[", "a", false, filepath.ErrBadPattern},
	{"[^", "a", false, filepath.ErrBadPattern},
	{"[^bc", "a", false, filepath.ErrBadPattern},
	{"a[", "a", false, filepath.ErrBadPattern}, // was nil but IMO its wrong
	{"a[", "ab", false, filepath.ErrBadPattern},
	{"*x", "xxx", true, nil},
}

func errp(e error) string {
	if e == nil {
		return "<nil>"
	}
	return e.Error()
}

// TestMatch tests our version of filepath.Match, called Matches.
func TestMatch(t *testing.T) {
	for _, tt := range matchTests {
		pattern := tt.pattern
		s := tt.s
		if runtime.GOOS == "windows" {
			if strings.Contains(pattern, "\\") {
				// no escape allowed on windows.
				continue
			}
			pattern = filepath.Clean(pattern)
			s = filepath.Clean(s)
		}
		ok, err := Matches(s, []string{pattern})
		if ok != tt.match || err != tt.err {
			t.Fatalf("Match(%#q, %#q) = %v, %q want %v, %q", pattern, s, ok, errp(err), tt.match, errp(tt.err))
		}
	}
}

type compileTestCase struct {
	pattern               string
	matchType             matchType
	compiledRegexp        string
	windowsCompiledRegexp string
}

var compileTests = []compileTestCase{
	{"*", regexpMatch, `^[^/]*$`, `^[^\\]*$`},
	{"file*", regexpMatch, `^file[^/]*$`, `^file[^\\]*$`},
	{"*file", regexpMatch, `^[^/]*file$`, `^[^\\]*file$`},
	{"a*/b", regexpMatch, `^a[^/]*/b$`, `^a[^\\]*\\b$`},
	{"**", suffixMatch, "", ""},
	{"**/**", regexpMatch, `^(.*/)?.*$`, `^(.*\\)?.*$`},
	{"dir/**", prefixMatch, "", ""},
	{"**/dir", suffixMatch, "", ""},
	{"**/dir2/*", regexpMatch, `^(.*/)?dir2/[^/]*$`, `^(.*\\)?dir2\\[^\\]*$`},
	{"**/dir2/**", regexpMatch, `^(.*/)?dir2/.*$`, `^(.*\\)?dir2\\.*$`},
	{"**file", suffixMatch, "", ""},
	{"**/file*txt", regexpMatch, `^(.*/)?file[^/]*txt$`, `^(.*\\)?file[^\\]*txt$`},
	{"**/**/*.txt", regexpMatch, `^(.*/)?(.*/)?[^/]*\.txt$`, `^(.*\\)?(.*\\)?[^\\]*\.txt$`},
	{"a[b-d]e", regexpMatch, `^a[b-d]e$`, `^a[b-d]e$`},
	{".*", regexpMatch, `^\.[^/]*$`, `^\.[^\\]*$`},
	{"abc.def", exactMatch, "", ""},
	{"abc?def", regexpMatch, `^abc[^/]def$`, `^abc[^\\]def$`},
	{"**/foo/bar", suffixMatch, "", ""},
	{"a(b)c/def", exactMatch, "", ""},
	{"a.|)$(}+{bc", exactMatch, "", ""},
	{"dist/proxy.py-2.4.0rc3.dev36+g08acad9-py3-none-any.whl", exactMatch, "", ""},
}

// TestCompile confirms that "compile" assigns the correct match type to a
// variety of test case patterns. If the match type is regexp, it also confirms
// that the compiled regexp matches the expected regexp.
func TestCompile(t *testing.T) {
	t.Run("slash", testCompile("/"))
	t.Run("backslash", testCompile(`\`))
}

func testCompile(sl string) func(*testing.T) {
	return func(t *testing.T) {
		for _, tt := range compileTests {
			// Avoid NewPatternMatcher, which has platform-specific behavior
			pm := &PatternMatcher{
				patterns: make([]*Pattern, 1),
			}
			pattern := path.Clean(tt.pattern)
			if sl != "/" {
				pattern = strings.ReplaceAll(pattern, "/", sl)
			}
			newp := &Pattern{}
			newp.cleanedPattern = pattern
			newp.dirs = strings.Split(pattern, sl)
			pm.patterns[0] = newp

			if err := pm.patterns[0].compile(sl); err != nil {
				t.Fatalf("Failed to compile pattern %q: %v", pattern, err)
			}
			if pm.patterns[0].matchType != tt.matchType {
				t.Errorf("pattern %q: matchType = %v, want %v", pattern, pm.patterns[0].matchType, tt.matchType)
				continue
			}
			if tt.matchType == regexpMatch {
				if sl == `\` {
					if pm.patterns[0].regexp.String() != tt.windowsCompiledRegexp {
						t.Errorf("pattern %q: regexp = %s, want %s", pattern, pm.patterns[0].regexp, tt.windowsCompiledRegexp)
					}
				} else if pm.patterns[0].regexp.String() != tt.compiledRegexp {
					t.Errorf("pattern %q: regexp = %s, want %s", pattern, pm.patterns[0].regexp, tt.compiledRegexp)
				}
			}
		}
	}
}
