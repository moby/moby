package chrootarchive

import (
	"bytes"
	"fmt"
	"hash/crc32"
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

func prepareSourceDirectory(numberOfFiles int, targetPath string, makeSymLinks bool) (int, error) {
	fileData := []byte("fooo")
	for n := 0; n < numberOfFiles; n++ {
		fileName := fmt.Sprintf("file-%d", n)
		if err := ioutil.WriteFile(path.Join(targetPath, fileName), fileData, 0700); err != nil {
			return 0, err
		}
		if makeSymLinks {
			if err := os.Symlink(path.Join(targetPath, fileName), path.Join(targetPath, fileName+"-link")); err != nil {
				return 0, err
			}
		}
	}
	totalSize := numberOfFiles * len(fileData)
	return totalSize, nil
}

func getHash(filename string) (uint32, error) {
	stream, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	hash := crc32.NewIEEE()
	hash.Write(stream)
	return hash.Sum32(), nil
}

func isSameType(fileInfo1, fileInfo2 os.FileInfo) bool {
	m1 := fileInfo1.Mode() & os.ModeType
	m2 := fileInfo2.Mode() & os.ModeType
	if m1 == m2 {
		return true
	}
	return false
}

func compare(src string, dest string) error {
	srcFileInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	destFileInfo, err := os.Lstat(dest)
	if err != nil {
		return err
	}

	if !isSameType(srcFileInfo, destFileInfo) {
		return fmt.Errorf("%s has a different file type from %s", src, dest)
	}
	if srcFileInfo.Mode().IsRegular() {
		err := compareFiles(src, dest)
		if err != nil {
			return err
		}
	} else if srcFileInfo.Mode().IsDir() {
		err := compareDirectories(src, dest)
		if err != nil {
			return err
		}
	} else if srcFileInfo.Mode()&os.ModeSymlink != 0 {
		err := compareSymlinks(src, dest)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Unsupported file type")
	}
	return nil
}

func compareDirectories(src string, dest string) error {
	changes, err := archive.ChangesDirs(dest, src)
	if err != nil {
		return err
	}
	if len(changes) > 0 {
		return fmt.Errorf("Compare %s and %s: expected 0 changes, got %d", src, dest, len(changes))
	}
	return nil
}

func compareFiles(src string, dest string) error {
	srcHash, err := getHash(src)
	if err != nil {
		return err
	}
	destHash, err := getHash(dest)
	if err != nil {
		return err
	}
	if srcHash != destHash {
		return fmt.Errorf("%s is different from %s", src, dest)
	}
	return nil
}

func compareSymlinks(src string, dest string) error {
	realSrc, err := os.Readlink(src)
	if err != nil {
		return err
	}
	realDest, err := os.Readlink(dest)
	if err != nil {
		return err
	}
	if err := compareFiles(realSrc, realDest); err != nil {
		return err
	}
	return nil
}

func TestChrootTarUntarWithSymlink(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "docker-TestChrootTarUntarWithSymlink")
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
	if err := compare(src, dest); err != nil {
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

	// Copy directory
	dest := filepath.Join(tmpdir, "dest")
	if err := CopyWithTar(src, dest); err != nil {
		t.Fatal(err)
	}
	if err := compare(src, dest); err != nil {
		t.Fatal(err)
	}

	// Copy file
	srcfile := filepath.Join(src, "file-1")
	dest = filepath.Join(tmpdir, "destFile")
	destfile := filepath.Join(dest, "file-1")
	if err := CopyWithTar(srcfile, destfile); err != nil {
		t.Fatal(err)
	}
	if err := compare(srcfile, destfile); err != nil {
		t.Fatal(err)
	}

	// FIXME: This test fails because CopyFileWithTar follows the symbolic link and
	// copies the regular file. It is unclear what the correct behavior should be.
	// Copy symbolic link
	/*srcLinkfile := filepath.Join(src, "file-1-link")
	dest = filepath.Join(tmpdir, "destSymlink")
	destLinkfile := filepath.Join(dest, "file-1-link")
	if err := CopyWithTar(srcLinkfile, destLinkfile); err != nil {
		t.Fatal(err)
	}
	if err := compare(srcLinkfile, destLinkfile); err != nil {
		t.Fatal(err)
	}*/
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

	// Copy directory
	dest := filepath.Join(tmpdir, "dest")
	if err := CopyFileWithTar(src, dest); err == nil {
		t.Fatal("Expected error on copying directory")
	}

	// Copy file
	srcfile := filepath.Join(src, "file-1")
	dest = filepath.Join(tmpdir, "destFile")
	destfile := filepath.Join(dest, "file-1")
	if err := CopyFileWithTar(srcfile, destfile); err != nil {
		t.Fatal(err)
	}
	if err := compare(srcfile, destfile); err != nil {
		t.Fatal(err)
	}

	// FIXME: This test fails because CopyFileWithTar follows the symbolic link and
	// copies the regular file. It is unclear what the correct behavior should be.
	// Copy symbolic link
	/*srcLinkfile := filepath.Join(src, "file-1-link")
	dest = filepath.Join(tmpdir, "destSymlink")
	destLinkfile := filepath.Join(dest, "file-1-link")
	if err := CopyFileWithTar(srcLinkfile, destLinkfile); err != nil {
		t.Fatal(err)
	}
	if err := compare(srcLinkfile, destLinkfile); err != nil {
		t.Fatal(err)
	}*/
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
	if err := compare(src, dest); err != nil {
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
