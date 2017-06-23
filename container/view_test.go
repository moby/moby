package container

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/registrar"
	"github.com/pborman/uuid"
)

var root string

func TestMain(m *testing.M) {
	var err error
	root, err = ioutil.TempDir("", "docker-container-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)

	os.Exit(m.Run())
}

func newContainer(t *testing.T) *Container {
	var (
		id    = uuid.New()
		cRoot = filepath.Join(root, id)
	)
	if err := os.MkdirAll(cRoot, 0755); err != nil {
		t.Fatal(err)
	}
	c := NewBaseContainer(id, cRoot)
	c.HostConfig = &containertypes.HostConfig{}
	return c
}

func TestViewSaveDelete(t *testing.T) {
	db, err := NewViewDB()
	if err != nil {
		t.Fatal(err)
	}
	c := newContainer(t)
	if err := c.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(c); err != nil {
		t.Fatal(err)
	}
}

func TestViewAll(t *testing.T) {
	var (
		db, _ = NewViewDB()
		names = registrar.NewRegistrar()
		one   = newContainer(t)
		two   = newContainer(t)
	)
	one.Pid = 10
	if err := one.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	two.Pid = 20
	if err := two.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}

	all, err := db.Snapshot(names).All()
	if err != nil {
		t.Fatal(err)
	}
	if l := len(all); l != 2 {
		t.Fatalf("expected 2 items, got %d", l)
	}
	byID := make(map[string]Snapshot)
	for i := range all {
		byID[all[i].ID] = all[i]
	}
	if s, ok := byID[one.ID]; !ok || s.Pid != 10 {
		t.Fatalf("expected something different with for id=%s: %v", one.ID, s)
	}
	if s, ok := byID[two.ID]; !ok || s.Pid != 20 {
		t.Fatalf("expected something different with for id=%s: %v", two.ID, s)
	}
}

func TestViewGet(t *testing.T) {
	var (
		db, _ = NewViewDB()
		names = registrar.NewRegistrar()
		one   = newContainer(t)
	)
	one.ImageID = "some-image-123"
	if err := one.CheckpointTo(db); err != nil {
		t.Fatal(err)
	}
	s, err := db.Snapshot(names).Get(one.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil || s.ImageID != "some-image-123" {
		t.Fatalf("expected ImageID=some-image-123. Got: %v", s)
	}
}
