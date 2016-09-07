package fileutils

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// CopyFile with invalid src
func (s *DockerSuite) TestCopyFileWithInvalidSrc(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	defer os.RemoveAll(tempFolder)
	if err != nil {
		c.Fatal(err)
	}
	bytes, err := CopyFile("/invalid/file/path", path.Join(tempFolder, "dest"))
	if err == nil {
		c.Fatal("Should have fail to copy an invalid src file")
	}
	if bytes != 0 {
		c.Fatal("Should have written 0 bytes")
	}

}

// CopyFile with invalid dest
func (s *DockerSuite) TestCopyFileWithInvalidDest(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	defer os.RemoveAll(tempFolder)
	if err != nil {
		c.Fatal(err)
	}
	src := path.Join(tempFolder, "file")
	err = ioutil.WriteFile(src, []byte("content"), 0740)
	if err != nil {
		c.Fatal(err)
	}
	bytes, err := CopyFile(src, path.Join(tempFolder, "/invalid/dest/path"))
	if err == nil {
		c.Fatal("Should have fail to copy an invalid src file")
	}
	if bytes != 0 {
		c.Fatal("Should have written 0 bytes")
	}

}

// CopyFile with same src and dest
func (s *DockerSuite) TestCopyFileWithSameSrcAndDest(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	defer os.RemoveAll(tempFolder)
	if err != nil {
		c.Fatal(err)
	}
	file := path.Join(tempFolder, "file")
	err = ioutil.WriteFile(file, []byte("content"), 0740)
	if err != nil {
		c.Fatal(err)
	}
	bytes, err := CopyFile(file, file)
	if err != nil {
		c.Fatal(err)
	}
	if bytes != 0 {
		c.Fatal("Should have written 0 bytes as it is the same file.")
	}
}

// CopyFile with same src and dest but path is different and not clean
func (s *DockerSuite) TestCopyFileWithSameSrcAndDestWithPathNameDifferent(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	defer os.RemoveAll(tempFolder)
	if err != nil {
		c.Fatal(err)
	}
	testFolder := path.Join(tempFolder, "test")
	err = os.MkdirAll(testFolder, 0740)
	if err != nil {
		c.Fatal(err)
	}
	file := path.Join(testFolder, "file")
	sameFile := testFolder + "/../test/file"
	err = ioutil.WriteFile(file, []byte("content"), 0740)
	if err != nil {
		c.Fatal(err)
	}
	bytes, err := CopyFile(file, sameFile)
	if err != nil {
		c.Fatal(err)
	}
	if bytes != 0 {
		c.Fatal("Should have written 0 bytes as it is the same file.")
	}
}

func (s *DockerSuite) TestCopyFile(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	defer os.RemoveAll(tempFolder)
	if err != nil {
		c.Fatal(err)
	}
	src := path.Join(tempFolder, "src")
	dest := path.Join(tempFolder, "dest")
	ioutil.WriteFile(src, []byte("content"), 0777)
	ioutil.WriteFile(dest, []byte("destContent"), 0777)
	bytes, err := CopyFile(src, dest)
	if err != nil {
		c.Fatal(err)
	}
	if bytes != 7 {
		c.Fatalf("Should have written %d bytes but wrote %d", 7, bytes)
	}
	actual, err := ioutil.ReadFile(dest)
	if err != nil {
		c.Fatal(err)
	}
	if string(actual) != "content" {
		c.Fatalf("Dest content was '%s', expected '%s'", string(actual), "content")
	}
}

// Reading a symlink to a directory must return the directory
func (s *DockerSuite) TestReadSymlinkedDirectoryExistingDirectory(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	var err error
	if err = os.Mkdir("/tmp/testReadSymlinkToExistingDirectory", 0777); err != nil {
		c.Errorf("failed to create directory: %s", err)
	}

	if err = os.Symlink("/tmp/testReadSymlinkToExistingDirectory", "/tmp/dirLinkTest"); err != nil {
		c.Errorf("failed to create symlink: %s", err)
	}

	var path string
	if path, err = ReadSymlinkedDirectory("/tmp/dirLinkTest"); err != nil {
		c.Fatalf("failed to read symlink to directory: %s", err)
	}

	if path != "/tmp/testReadSymlinkToExistingDirectory" {
		c.Fatalf("symlink returned unexpected directory: %s", path)
	}

	if err = os.Remove("/tmp/testReadSymlinkToExistingDirectory"); err != nil {
		c.Errorf("failed to remove temporary directory: %s", err)
	}

	if err = os.Remove("/tmp/dirLinkTest"); err != nil {
		c.Errorf("failed to remove symlink: %s", err)
	}
}

// Reading a non-existing symlink must fail
func (s *DockerSuite) TestReadSymlinkedDirectoryNonExistingSymlink(c *check.C) {
	var path string
	var err error
	if path, err = ReadSymlinkedDirectory("/tmp/test/foo/Non/ExistingPath"); err == nil {
		c.Fatalf("error expected for non-existing symlink")
	}

	if path != "" {
		c.Fatalf("expected empty path, but '%s' was returned", path)
	}
}

// Reading a symlink to a file must fail
func (s *DockerSuite) TestReadSymlinkedDirectoryToFile(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	var err error
	var file *os.File

	if file, err = os.Create("/tmp/testReadSymlinkToFile"); err != nil {
		c.Fatalf("failed to create file: %s", err)
	}

	file.Close()

	if err = os.Symlink("/tmp/testReadSymlinkToFile", "/tmp/fileLinkTest"); err != nil {
		c.Errorf("failed to create symlink: %s", err)
	}

	var path string
	if path, err = ReadSymlinkedDirectory("/tmp/fileLinkTest"); err == nil {
		c.Fatalf("ReadSymlinkedDirectory on a symlink to a file should've failed")
	}

	if path != "" {
		c.Fatalf("path should've been empty: %s", path)
	}

	if err = os.Remove("/tmp/testReadSymlinkToFile"); err != nil {
		c.Errorf("failed to remove file: %s", err)
	}

	if err = os.Remove("/tmp/fileLinkTest"); err != nil {
		c.Errorf("failed to remove symlink: %s", err)
	}
}

func (s *DockerSuite) TestWildcardMatches(c *check.C) {
	match, _ := Matches("fileutils.go", []string{"*"})
	if match != true {
		c.Errorf("failed to get a wildcard match, got %v", match)
	}
}

// A simple pattern match should return true.
func (s *DockerSuite) TestPatternMatches(c *check.C) {
	match, _ := Matches("fileutils.go", []string{"*.go"})
	if match != true {
		c.Errorf("failed to get a match, got %v", match)
	}
}

// An exclusion followed by an inclusion should return true.
func (s *DockerSuite) TestExclusionPatternMatchesPatternBefore(c *check.C) {
	match, _ := Matches("fileutils.go", []string{"!fileutils.go", "*.go"})
	if match != true {
		c.Errorf("failed to get true match on exclusion pattern, got %v", match)
	}
}

// A folder pattern followed by an exception should return false.
func (s *DockerSuite) TestPatternMatchesFolderExclusions(c *check.C) {
	match, _ := Matches("docs/README.md", []string{"docs", "!docs/README.md"})
	if match != false {
		c.Errorf("failed to get a false match on exclusion pattern, got %v", match)
	}
}

// A folder pattern followed by an exception should return false.
func (s *DockerSuite) TestPatternMatchesFolderWithSlashExclusions(c *check.C) {
	match, _ := Matches("docs/README.md", []string{"docs/", "!docs/README.md"})
	if match != false {
		c.Errorf("failed to get a false match on exclusion pattern, got %v", match)
	}
}

// A folder pattern followed by an exception should return false.
func (s *DockerSuite) TestPatternMatchesFolderWildcardExclusions(c *check.C) {
	match, _ := Matches("docs/README.md", []string{"docs/*", "!docs/README.md"})
	if match != false {
		c.Errorf("failed to get a false match on exclusion pattern, got %v", match)
	}
}

// A pattern followed by an exclusion should return false.
func (s *DockerSuite) TestExclusionPatternMatchesPatternAfter(c *check.C) {
	match, _ := Matches("fileutils.go", []string{"*.go", "!fileutils.go"})
	if match != false {
		c.Errorf("failed to get false match on exclusion pattern, got %v", match)
	}
}

// A filename evaluating to . should return false.
func (s *DockerSuite) TestExclusionPatternMatchesWholeDirectory(c *check.C) {
	match, _ := Matches(".", []string{"*.go"})
	if match != false {
		c.Errorf("failed to get false match on ., got %v", match)
	}
}

// A single ! pattern should return an error.
func (s *DockerSuite) TestSingleExclamationError(c *check.C) {
	_, err := Matches("fileutils.go", []string{"!"})
	if err == nil {
		c.Errorf("failed to get an error for a single exclamation point, got %v", err)
	}
}

// A string preceded with a ! should return true from Exclusion.
func (s *DockerSuite) TestExclusion(c *check.C) {
	exclusion := exclusion("!")
	if !exclusion {
		c.Errorf("failed to get true for a single !, got %v", exclusion)
	}
}

// Matches with no patterns
func (s *DockerSuite) TestMatchesWithNoPatterns(c *check.C) {
	matches, err := Matches("/any/path/there", []string{})
	if err != nil {
		c.Fatal(err)
	}
	if matches {
		c.Fatalf("Should not have match anything")
	}
}

// Matches with malformed patterns
func (s *DockerSuite) TestMatchesWithMalformedPatterns(c *check.C) {
	matches, err := Matches("/any/path/there", []string{"["})
	if err == nil {
		c.Fatal("Should have failed because of a malformed syntax in the pattern")
	}
	if matches {
		c.Fatalf("Should not have match anything")
	}
}

// Test lots of variants of patterns & strings
func (s *DockerSuite) TestMatches(c *check.C) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		c.Skip("Needs porting to Windows")
	}
	tests := []struct {
		pattern string
		text    string
		pass    bool
	}{
		{"**", "file", true},
		{"**", "file/", true},
		{"**/", "file", true}, // weird one
		{"**/", "file/", true},
		{"**", "/", true},
		{"**/", "/", true},
		{"**", "dir/file", true},
		{"**/", "dir/file", false},
		{"**", "dir/file/", true},
		{"**/", "dir/file/", true},
		{"**/**", "dir/file", true},
		{"**/**", "dir/file/", true},
		{"dir/**", "dir/file", true},
		{"dir/**", "dir/file/", true},
		{"dir/**", "dir/dir2/file", true},
		{"dir/**", "dir/dir2/file/", true},
		{"**/dir2/*", "dir/dir2/file", true},
		{"**/dir2/*", "dir/dir2/file/", false},
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
		{"a\\*b", "a*b", true},
		{"a\\", "a", false},
		{"a\\", "a\\", false},
		{"a\\\\", "a\\", true},
		{"**/foo/bar", "foo/bar", true},
		{"**/foo/bar", "dir/foo/bar", true},
		{"**/foo/bar", "dir/dir2/foo/bar", true},
		{"abc/**", "abc", false},
		{"abc/**", "abc/def", true},
		{"abc/**", "abc/def/ghi", true},
	}

	for _, test := range tests {
		res, _ := regexpMatch(test.pattern, test.text)
		if res != test.pass {
			c.Fatalf("Failed: %v - res:%v", test, res)
		}
	}
}

// An empty string should return true from Empty.
func (s *DockerSuite) TestEmpty(c *check.C) {
	empty := empty("")
	if !empty {
		c.Errorf("failed to get true for an empty string, got %v", empty)
	}
}

func (s *DockerSuite) TestCleanPatterns(c *check.C) {
	cleaned, _, _, _ := CleanPatterns([]string{"docs", "config"})
	if len(cleaned) != 2 {
		c.Errorf("expected 2 element slice, got %v", len(cleaned))
	}
}

func (s *DockerSuite) TestCleanPatternsStripEmptyPatterns(c *check.C) {
	cleaned, _, _, _ := CleanPatterns([]string{"docs", "config", ""})
	if len(cleaned) != 2 {
		c.Errorf("expected 2 element slice, got %v", len(cleaned))
	}
}

func (s *DockerSuite) TestCleanPatternsExceptionFlag(c *check.C) {
	_, _, exceptions, _ := CleanPatterns([]string{"docs", "!docs/README.md"})
	if !exceptions {
		c.Errorf("expected exceptions to be true, got %v", exceptions)
	}
}

func (s *DockerSuite) TestCleanPatternsLeadingSpaceTrimmed(c *check.C) {
	_, _, exceptions, _ := CleanPatterns([]string{"docs", "  !docs/README.md"})
	if !exceptions {
		c.Errorf("expected exceptions to be true, got %v", exceptions)
	}
}

func (s *DockerSuite) TestCleanPatternsTrailingSpaceTrimmed(c *check.C) {
	_, _, exceptions, _ := CleanPatterns([]string{"docs", "!docs/README.md  "})
	if !exceptions {
		c.Errorf("expected exceptions to be true, got %v", exceptions)
	}
}

func (s *DockerSuite) TestCleanPatternsErrorSingleException(c *check.C) {
	_, _, _, err := CleanPatterns([]string{"!"})
	if err == nil {
		c.Errorf("expected error on single exclamation point, got %v", err)
	}
}

func (s *DockerSuite) TestCleanPatternsFolderSplit(c *check.C) {
	_, dirs, _, _ := CleanPatterns([]string{"docs/config/CONFIG.md"})
	if dirs[0][0] != "docs" {
		c.Errorf("expected first element in dirs slice to be docs, got %v", dirs[0][1])
	}
	if dirs[0][1] != "config" {
		c.Errorf("expected first element in dirs slice to be config, got %v", dirs[0][1])
	}
}

func (s *DockerSuite) TestCreateIfNotExistsDir(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tempFolder)

	folderToCreate := filepath.Join(tempFolder, "tocreate")

	if err := CreateIfNotExists(folderToCreate, true); err != nil {
		c.Fatal(err)
	}
	fileinfo, err := os.Stat(folderToCreate)
	if err != nil {
		c.Fatalf("Should have create a folder, got %v", err)
	}

	if !fileinfo.IsDir() {
		c.Fatalf("Should have been a dir, seems it's not")
	}
}

func (s *DockerSuite) TestCreateIfNotExistsFile(c *check.C) {
	tempFolder, err := ioutil.TempDir("", "docker-fileutils-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tempFolder)

	fileToCreate := filepath.Join(tempFolder, "file/to/create")

	if err := CreateIfNotExists(fileToCreate, false); err != nil {
		c.Fatal(err)
	}
	fileinfo, err := os.Stat(fileToCreate)
	if err != nil {
		c.Fatalf("Should have create a file, got %v", err)
	}

	if fileinfo.IsDir() {
		c.Fatalf("Should have been a file, seems it's not")
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
	{"a*", "ab/c", false, nil},
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

// TestMatch test's our version of filepath.Match, called regexpMatch.
func (s *DockerSuite) TestMatch(c *check.C) {
	for _, tt := range matchTests {
		pattern := tt.pattern
		st := tt.s
		if runtime.GOOS == "windows" {
			if strings.Index(pattern, "\\") >= 0 {
				// no escape allowed on windows.
				continue
			}
			pattern = filepath.Clean(pattern)
			st = filepath.Clean(st)
		}
		ok, err := regexpMatch(pattern, st)
		if ok != tt.match || err != tt.err {
			c.Fatalf("Match(%#q, %#q) = %v, %q want %v, %q", pattern, st, ok, errp(err), tt.match, errp(tt.err))
		}
	}
}
