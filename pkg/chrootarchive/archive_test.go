package chrootarchive

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
)

func init() {
	reexec.Init()
}

func TestChrootTarUntar(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootTarUntar")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	src := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(src, "toto"), []byte("hello toto"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(src, "lolo"), []byte("hello lolo"), 0644); err != nil {
		t.Fatal(err)
	}
	stream, err := archive.Tar(src, archive.Uncompressed)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Untar(stream, dest, &archive.TarOptions{ExcludePatterns: []string{"lolo"}}); err != nil {
		t.Fatal(err)
	}
}

func TestChrootUntarEmptyArchive(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootUntarEmptyArchive")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	if err := Untar(nil, tmpdir, nil); err == nil {
		t.Fatal("expected error on empty archive")
	}
}

func prepareSourceDirectory(numberOfFiles int, targetPath string, makeLinks bool) (int, error) {
	fileData := []byte("fooo")
	for n := 0; n < numberOfFiles; n++ {
		fileName := fmt.Sprintf("file-%d", n)
		if err := ioutil.WriteFile(path.Join(targetPath, fileName), fileData, 0700); err != nil {
			return 0, err
		}
		if makeLinks {
			if err := os.Link(path.Join(targetPath, fileName), path.Join(targetPath, fileName+"-link")); err != nil {
				return 0, err
			}
		}
	}
	totalSize := numberOfFiles * len(fileData)
	return totalSize, nil
}

func TestChrootTarUntarWithSoftLink(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootTarUntarWithSoftLink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	src := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSourceDirectory(10, src, true); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpdir, "dest")
	if err := TarUntar(src, dest); err != nil {
		t.Fatal(err)
	}
}

func TestChrootCopyWithTar(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootCopyWithTar")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	src := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSourceDirectory(10, src, true); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpdir, "dest")
	// Copy directory
	if err := CopyWithTar(src, dest); err != nil {
		t.Fatal(err)
	}
	// Copy file
	srcfile := filepath.Join(src, "file-1")
	if err := CopyWithTar(srcfile, dest); err != nil {
		t.Fatal(err)
	}
	// Copy symbolic link
	linkfile := filepath.Join(src, "file-1-link")
	if err := CopyWithTar(linkfile, dest); err != nil {
		t.Fatal(err)
	}
}

func TestChrootCopyFileWithTar(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootCopyFileWithTar")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	src := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSourceDirectory(10, src, true); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpdir, "dest")
	// Copy directory
	if err := CopyFileWithTar(src, dest); err == nil {
		t.Fatal("Expected error on copying directory")
	}
	// Copy file
	srcfile := filepath.Join(src, "file-1")
	if err := CopyFileWithTar(srcfile, dest); err != nil {
		t.Fatal(err)
	}
	// Copy symbolic link
	linkfile := filepath.Join(src, "file-1-link")
	if err := CopyFileWithTar(linkfile, dest); err != nil {
		t.Fatal(err)
	}
}

func TestChrootUntarPath(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootUntarPath")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	src := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSourceDirectory(10, src, true); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpdir, "dest")
	// Untar a directory
	if err := UntarPath(src, dest); err == nil {
		t.Fatal("Expected error on untaring a directory")
	}

	// Untar a tar file
	stream, err := archive.Tar(src, archive.Uncompressed)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(stream)
	tarfile := filepath.Join(tmpdir, "src.tar")
	if err := ioutil.WriteFile(tarfile, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := UntarPath(tarfile, dest); err != nil {
		t.Fatal(err)
	}
}

type slowEmptyTarReader struct {
	size      int
	offset    int
	chunkSize int
}

// Read is a slow reader of an empty tar (like the output of "tar c --files-from /dev/null")
func (s *slowEmptyTarReader) Read(p []byte) (int, error) {
	time.Sleep(100 * time.Millisecond)
	count := s.chunkSize
	if len(p) < s.chunkSize {
		count = len(p)
	}
	for i := 0; i < count; i++ {
		p[i] = 0
	}
	s.offset += count
	if s.offset > s.size {
		return count, io.EOF
	}
	return count, nil
}

func TestChrootUntarEmptyArchiveFromSlowReader(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootUntarEmptyArchiveFromSlowReader")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	dest := filepath.Join(tmpdir, "dest")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	stream := &slowEmptyTarReader{size: 10240, chunkSize: 1024}
	if err := Untar(stream, dest, nil); err != nil {
		t.Fatal(err)
	}
}

func TestChrootApplyEmptyArchiveFromSlowReader(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootApplyEmptyArchiveFromSlowReader")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	dest := filepath.Join(tmpdir, "dest")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	stream := &slowEmptyTarReader{size: 10240, chunkSize: 1024}
	if _, err := ApplyLayer(dest, stream); err != nil {
		t.Fatal(err)
	}
}

func TestChrootApplyDotDotFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootApplyDotDotFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	src := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(src, 0700); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(src, "..gitme"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	stream, err := archive.Tar(src, archive.Uncompressed)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmpdir, "dest")
	if err := os.MkdirAll(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyLayer(dest, stream); err != nil {
		t.Fatal(err)
	}
}
