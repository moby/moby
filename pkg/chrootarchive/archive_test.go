package chrootarchive

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

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
	if err := Untar(stream, dest, &archive.TarOptions{Excludes: []string{"lolo"}}); err != nil {
		t.Fatal(err)
	}
}
