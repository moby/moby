package chrootarchive

import (
	"io"
	"io/ioutil"
	"os"
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
