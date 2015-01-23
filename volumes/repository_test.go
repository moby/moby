package volumes

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/vfs"
)

func TestRepositoryFindOrCreate(t *testing.T) {
	root, err := ioutil.TempDir(os.TempDir(), "volumes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	repo, err := newRepo(root)
	if err != nil {
		t.Fatal(err)
	}

	// no path
	v, err := repo.FindOrCreateVolume("", true)
	if err != nil {
		t.Fatal(err)
	}

	// FIXME: volumes are heavily dependent on the vfs driver, but this should not be so!
	expected := filepath.Join(root, "repo-graph", "vfs", "dir", v.ID)
	if v.Path != expected {
		t.Fatalf("expected new path to be created in %s, got %s", expected, v.Path)
	}

	// with a non-existant path
	dir := filepath.Join(root, "doesntexist")
	v, err = repo.FindOrCreateVolume(dir, true)
	if err != nil {
		t.Fatal(err)
	}

	if v.Path != dir {
		t.Fatalf("expected new path to be created in %s, got %s", dir, v.Path)
	}

	if _, err := os.Stat(v.Path); err != nil {
		t.Fatal(err)
	}

	// with a pre-existing path
	// can just use the same path from above since it now exists
	v, err = repo.FindOrCreateVolume(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if v.Path != dir {
		t.Fatalf("expected new path to be created in %s, got %s", dir, v.Path)
	}

}

func TestRepositoryGet(t *testing.T) {
	root, err := ioutil.TempDir(os.TempDir(), "volumes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	repo, err := newRepo(root)
	if err != nil {
		t.Fatal(err)
	}

	v, err := repo.FindOrCreateVolume("", true)
	if err != nil {
		t.Fatal(err)
	}

	v2 := repo.Get(v.Path)
	if v2 == nil {
		t.Fatalf("expected to find volume but didn't")
	}
	if v2 != v {
		t.Fatalf("expected get to return same volume")
	}
}

func TestRepositoryDelete(t *testing.T) {
	root, err := ioutil.TempDir(os.TempDir(), "volumes")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	repo, err := newRepo(root)
	if err != nil {
		t.Fatal(err)
	}

	// with a normal volume
	v, err := repo.FindOrCreateVolume("", true)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(v.Path); err != nil {
		t.Fatal(err)
	}

	if v := repo.Get(v.Path); v != nil {
		t.Fatalf("expected volume to not exist")
	}

	if _, err := os.Stat(v.Path); err == nil {
		t.Fatalf("expected volume files to be removed")
	}

	// with a bind mount
	dir := filepath.Join(root, "test")
	v, err = repo.FindOrCreateVolume(dir, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(v.Path); err != nil {
		t.Fatal(err)
	}

	if v := repo.Get(v.Path); v != nil {
		t.Fatalf("expected volume to not exist")
	}

	if _, err := os.Stat(v.Path); err != nil && os.IsNotExist(err) {
		t.Fatalf("expected bind volume data to persist after destroying volume")
	}

	// with container refs
	dir = filepath.Join(root, "test")
	v, err = repo.FindOrCreateVolume(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	v.AddContainer("1234")

	if err := repo.Delete(v.Path); err == nil {
		t.Fatalf("expected volume delete to fail due to container refs")
	}

	v.RemoveContainer("1234")
	if err := repo.Delete(v.Path); err != nil {
		t.Fatal(err)
	}

}

func newRepo(root string) (*Repository, error) {
	configPath := filepath.Join(root, "repo-config")
	graphDir := filepath.Join(root, "repo-graph")

	driver, err := graphdriver.GetDriver("vfs", graphDir, []string{})
	if err != nil {
		return nil, err
	}
	return NewRepository(configPath, driver)
}
