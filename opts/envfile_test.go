package opts

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
)

func tmpFileWithContent(content string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "envfile-test")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	tmpFile.WriteString(content)
	return tmpFile.Name(), nil
}

// Test ParseEnvFile for a file with a few well formatted lines
func TestParseEnvFileGoodFile(t *testing.T) {
	content := `foo=bar
    baz=quux
# comment

foobar=foobaz
`

	tmpFile, err := tmpFileWithContent(content)
	if err != nil {
		t.Fatal("failed to create test data file")
	}
	defer os.Remove(tmpFile)

	lines, err := ParseEnvFile(tmpFile)
	if err != nil {
		t.Fatal("ParseEnvFile failed; expected success")
	}

	expected_lines := []string{
		"foo=bar",
		"baz=quux",
		"foobar=foobaz",
	}

	if !reflect.DeepEqual(lines, expected_lines) {
		t.Fatal("lines not equal to expected_lines")
	}
}

// Test ParseEnvFile for an empty file
func TestParseEnvFileEmptyFile(t *testing.T) {
	tmpFile, err := tmpFileWithContent("")
	if err != nil {
		t.Fatal("failed to create test data file")
	}
	defer os.Remove(tmpFile)

	lines, err := ParseEnvFile(tmpFile)
	if err != nil {
		t.Fatal("ParseEnvFile failed; expected success")
	}

	if len(lines) != 0 {
		t.Fatal("lines not empty; expected empty")
	}
}

// Test ParseEnvFile for a non existent file
func TestParseEnvFileNonExistentFile(t *testing.T) {
	_, err := ParseEnvFile("foo_bar_baz")
	if err == nil {
		t.Fatal("ParseEnvFile succeeded; expected failure")
	}
}

// Test ParseEnvFile for a badly formatted file
func TestParseEnvFileBadlyFormattedFile(t *testing.T) {
	content := `foo=bar
    f   =quux
`

	tmpFile, err := tmpFileWithContent(content)
	if err != nil {
		t.Fatal("failed to create test data file")
	}
	defer os.Remove(tmpFile)

	_, err = ParseEnvFile(tmpFile)
	if err == nil {
		t.Fatal("ParseEnvFile succeeded; expected failure")
	}
}

// Test ParseEnvFile for a file with a line exeeding bufio.MaxScanTokenSize
func TestParseEnvFileLineTooLongFile(t *testing.T) {
	content := strings.Repeat("a", bufio.MaxScanTokenSize+42)
	content = fmt.Sprint("foo=", content)

	tmpFile, err := tmpFileWithContent(content)
	if err != nil {
		t.Fatal("failed to create test data file")
	}
	defer os.Remove(tmpFile)

	_, err = ParseEnvFile(tmpFile)
	if err == nil {
		t.Fatal("ParseEnvFile succeeded; expected failure")
	}
}
