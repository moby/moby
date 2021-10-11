package stack // import "github.com/docker/docker/pkg/stack"

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDump(t *testing.T) {
	Dump()
}

func TestDumpToFile(t *testing.T) {
	directory, err := os.MkdirTemp("", "test-dump-tasks")
	assert.Check(t, err)
	defer os.RemoveAll(directory)
	dumpPath, err := DumpToFile(directory)
	assert.Check(t, err)
	readFile, _ := os.ReadFile(dumpPath)
	fileData := string(readFile)
	assert.Check(t, is.Contains(fileData, "goroutine"))
}

func TestDumpToFileWithEmptyInput(t *testing.T) {
	path, err := DumpToFile("")
	assert.Check(t, err)
	assert.Check(t, is.Equal(os.Stderr.Name(), path))
}
