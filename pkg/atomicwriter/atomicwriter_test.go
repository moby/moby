package atomicwriter

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// testMode returns the file-mode to use in tests, accounting for Windows
// not supporting full Linux file mode.
func testMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o666
	}
	return 0o640
}

// assertFile asserts the given fileName to exist, and to have the expected
// content and mode.
func assertFile(t *testing.T, fileName string, fileContent []byte, expectedMode os.FileMode) {
	t.Helper()
	actual, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("Error reading from file: %v", err)
	}

	if !bytes.Equal(actual, fileContent) {
		t.Errorf("Data mismatch, expected %q, got %q", fileContent, actual)
	}

	st, err := os.Stat(fileName)
	if err != nil {
		t.Fatalf("Error statting file: %v", err)
	}
	if st.Mode() != expectedMode {
		t.Errorf("Mode mismatched, expected %o, got %o", expectedMode, st.Mode())
	}
}

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()

	fileName := filepath.Join(tmpDir, "test.txt")
	fileContent := []byte("file content")
	fileMode := testMode()
	if err := WriteFile(fileName, fileContent, fileMode); err != nil {
		t.Fatalf("Error writing to file: %v", err)
	}
	assertFile(t, fileName, fileContent, fileMode)
}

func TestWriteSetCommit(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.Mkdir(filepath.Join(tmpDir, "tmp"), 0o700); err != nil {
		t.Fatalf("Error creating tmp directory: %s", err)
	}

	targetDir := filepath.Join(tmpDir, "target")
	ws, err := NewWriteSet(filepath.Join(tmpDir, "tmp"))
	if err != nil {
		t.Fatalf("Error creating atomic write set: %s", err)
	}

	fileContent := []byte("file content")
	fileMode := testMode()

	if err := ws.WriteFile("foo", fileContent, fileMode); err != nil {
		t.Fatalf("Error writing to file: %v", err)
	}

	if _, err := os.ReadFile(filepath.Join(targetDir, "foo")); err == nil {
		t.Fatalf("Expected error reading file where should not exist")
	}

	if err := ws.Commit(targetDir); err != nil {
		t.Fatalf("Error committing file: %s", err)
	}

	assertFile(t, filepath.Join(targetDir, "foo"), fileContent, fileMode)
}

func TestWriteSetCancel(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.Mkdir(filepath.Join(tmpDir, "tmp"), 0o700); err != nil {
		t.Fatalf("Error creating tmp directory: %s", err)
	}

	ws, err := NewWriteSet(filepath.Join(tmpDir, "tmp"))
	if err != nil {
		t.Fatalf("Error creating atomic write set: %s", err)
	}

	fileContent := []byte("file content")
	fileMode := testMode()
	if err := ws.WriteFile("foo", fileContent, fileMode); err != nil {
		t.Fatalf("Error writing to file: %v", err)
	}

	if err := ws.Cancel(); err != nil {
		t.Fatalf("Error committing file: %s", err)
	}

	if _, err := os.ReadFile(filepath.Join(tmpDir, "target", "foo")); err == nil {
		t.Fatalf("Expected error reading file where should not exist")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Unexpected error reading file: %s", err)
	}
}
