package remotecontext // import "github.com/docker/docker/builder/remotecontext"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/pkg/errors"
	"gotest.tools/v3/skip"
)

const (
	filename = "test"
	contents = "contents test"
)

func init() {
	reexec.Init()
}

func TestCloseRootDirectory(t *testing.T) {
	contextDir, err := os.MkdirTemp("", "builder-tarsum-test")
	defer os.RemoveAll(contextDir)
	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	src := makeTestArchiveContext(t, contextDir)
	err = src.Close()

	if err != nil {
		t.Fatalf("Error while executing Close: %s", err)
	}

	_, err = os.Stat(src.Root().Path())

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Directory should not exist at this point")
	}
}

func TestHashFile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	createTestTempFile(t, contextDir, filename, contents, 0755)

	tarSum := makeTestArchiveContext(t, contextDir)

	sum, err := tarSum.Hash(filename)

	if err != nil {
		t.Fatalf("Error when executing Stat: %s", err)
	}

	if len(sum) == 0 {
		t.Fatalf("Hash returned empty sum")
	}

	expected := "55dfeb344351ab27f59aa60ebb0ed12025a2f2f4677bf77d26ea7a671274a9ca"

	if actual := sum; expected != actual {
		t.Fatalf("invalid checksum. expected %s, got %s", expected, actual)
	}
}

func TestHashSubdir(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := filepath.Join(contextDir, "builder-tarsum-test-subdir")
	err := os.Mkdir(contextSubdir, 0755)
	if err != nil {
		t.Fatalf("Failed to make directory: %s", contextSubdir)
	}

	testFilename := createTestTempFile(t, contextSubdir, filename, contents, 0755)

	tarSum := makeTestArchiveContext(t, contextDir)

	relativePath, err := filepath.Rel(contextDir, testFilename)

	if err != nil {
		t.Fatalf("Error when getting relative path: %s", err)
	}

	sum, err := tarSum.Hash(relativePath)

	if err != nil {
		t.Fatalf("Error when executing Stat: %s", err)
	}

	if len(sum) == 0 {
		t.Fatalf("Hash returned empty sum")
	}

	expected := "74a3326b8e766ce63a8e5232f22e9dd895be647fb3ca7d337e5e0a9b3da8ef28"

	if actual := sum; expected != actual {
		t.Fatalf("invalid checksum. expected %s, got %s", expected, actual)
	}
}

func TestRemoveDirectory(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(t, contextDir, "builder-tarsum-test-subdir")

	relativePath, err := filepath.Rel(contextDir, contextSubdir)

	if err != nil {
		t.Fatalf("Error when getting relative path: %s", err)
	}

	src := makeTestArchiveContext(t, contextDir)

	_, err = os.Stat(filepath.Join(src.Root().Path(), relativePath))
	if err != nil {
		t.Fatalf("Statting %s shouldn't fail: %+v", relativePath, err)
	}

	tarSum := src.(modifiableContext)
	err = tarSum.Remove(relativePath)
	if err != nil {
		t.Fatalf("Error when executing Remove: %s", err)
	}

	_, err = os.Stat(filepath.Join(src.Root().Path(), relativePath))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Directory should not exist at this point: %+v ", err)
	}
}

func makeTestArchiveContext(t *testing.T, dir string) builder.Source {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	tarStream, err := archive.Tar(dir, archive.Uncompressed)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	defer tarStream.Close()
	tarSum, err := FromArchive(tarStream)
	if err != nil {
		t.Fatalf("Error when executing FromArchive: %s", err)
	}
	return tarSum
}
