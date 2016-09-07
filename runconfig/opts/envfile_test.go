package opts

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func tmpFileWithContent(content string, c *check.C) string {
	tmpFile, err := ioutil.TempFile("", "envfile-test")
	if err != nil {
		c.Fatal(err)
	}
	defer tmpFile.Close()

	tmpFile.WriteString(content)
	return tmpFile.Name()
}

// Test ParseEnvFile for a file with a few well formatted lines
func (s *DockerSuite) TestParseEnvFileGoodFile(c *check.C) {
	content := `foo=bar
    baz=quux
# comment

_foobar=foobaz
with.dots=working
and_underscore=working too
`
	// Adding a newline + a line with pure whitespace.
	// This is being done like this instead of the block above
	// because it's common for editors to trim trailing whitespace
	// from lines, which becomes annoying since that's the
	// exact thing we need to test.
	content += "\n    \t  "
	tmpFile := tmpFileWithContent(content, c)
	defer os.Remove(tmpFile)

	lines, err := ParseEnvFile(tmpFile)
	if err != nil {
		c.Fatal(err)
	}

	expectedLines := []string{
		"foo=bar",
		"baz=quux",
		"_foobar=foobaz",
		"with.dots=working",
		"and_underscore=working too",
	}

	if !reflect.DeepEqual(lines, expectedLines) {
		c.Fatal("lines not equal to expected_lines")
	}
}

// Test ParseEnvFile for an empty file
func (s *DockerSuite) TestParseEnvFileEmptyFile(c *check.C) {
	tmpFile := tmpFileWithContent("", c)
	defer os.Remove(tmpFile)

	lines, err := ParseEnvFile(tmpFile)
	if err != nil {
		c.Fatal(err)
	}

	if len(lines) != 0 {
		c.Fatal("lines not empty; expected empty")
	}
}

// Test ParseEnvFile for a non existent file
func (s *DockerSuite) TestParseEnvFileNonExistentFile(c *check.C) {
	_, err := ParseEnvFile("foo_bar_baz")
	if err == nil {
		c.Fatal("ParseEnvFile succeeded; expected failure")
	}
	if _, ok := err.(*os.PathError); !ok {
		c.Fatalf("Expected a PathError, got [%v]", err)
	}
}

// Test ParseEnvFile for a badly formatted file
func (s *DockerSuite) TestParseEnvFileBadlyFormattedFile(c *check.C) {
	content := `foo=bar
    f   =quux
`

	tmpFile := tmpFileWithContent(content, c)
	defer os.Remove(tmpFile)

	_, err := ParseEnvFile(tmpFile)
	if err == nil {
		c.Fatalf("Expected an ErrBadEnvVariable, got nothing")
	}
	if _, ok := err.(ErrBadEnvVariable); !ok {
		c.Fatalf("Expected an ErrBadEnvVariable, got [%v]", err)
	}
	expectedMessage := "poorly formatted environment: variable 'f   ' has white spaces"
	if err.Error() != expectedMessage {
		c.Fatalf("Expected [%v], got [%v]", expectedMessage, err.Error())
	}
}

// Test ParseEnvFile for a file with a line exceeding bufio.MaxScanTokenSize
func (s *DockerSuite) TestParseEnvFileLineTooLongFile(c *check.C) {
	content := strings.Repeat("a", bufio.MaxScanTokenSize+42)
	content = fmt.Sprint("foo=", content)

	tmpFile := tmpFileWithContent(content, c)
	defer os.Remove(tmpFile)

	_, err := ParseEnvFile(tmpFile)
	if err == nil {
		c.Fatal("ParseEnvFile succeeded; expected failure")
	}
}

// ParseEnvFile with a random file, pass through
func (s *DockerSuite) TestParseEnvFileRandomFile(c *check.C) {
	content := `first line
another invalid line`
	tmpFile := tmpFileWithContent(content, c)
	defer os.Remove(tmpFile)

	_, err := ParseEnvFile(tmpFile)

	if err == nil {
		c.Fatalf("Expected an ErrBadEnvVariable, got nothing")
	}
	if _, ok := err.(ErrBadEnvVariable); !ok {
		c.Fatalf("Expected an ErrBadEnvvariable, got [%v]", err)
	}
	expectedMessage := "poorly formatted environment: variable 'first line' has white spaces"
	if err.Error() != expectedMessage {
		c.Fatalf("Expected [%v], got [%v]", expectedMessage, err.Error())
	}
}
