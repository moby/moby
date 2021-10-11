//go:build linux || freebsd
// +build linux freebsd

package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"testing"
)

// TestLstat tests Lstat for existing and non existing files
func TestLstat(t *testing.T) {
	file, invalid, _, dir := prepareFiles(t)
	defer os.RemoveAll(dir)

	statFile, err := Lstat(file)
	if err != nil {
		t.Fatal(err)
	}
	if statFile == nil {
		t.Fatal("returned empty stat for existing file")
	}

	statInvalid, err := Lstat(invalid)
	if err == nil {
		t.Fatal("did not return error for non-existing file")
	}
	if statInvalid != nil {
		t.Fatal("returned non-nil stat for non-existing file")
	}
}
