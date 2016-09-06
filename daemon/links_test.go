package daemon

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/stringid"
)

func TestMigrateLegacySqliteLinks(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "legacy-qlite-links-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	name1 := "test1"
	c1 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:         stringid.GenerateNonCryptoID(),
			Name:       name1,
			HostConfig: &containertypes.HostConfig{},
		},
	}
	c1.Root = tmpDir

	name2 := "test2"
	c2 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   stringid.GenerateNonCryptoID(),
			Name: name2,
		},
	}

	store := container.NewMemoryStore()
	store.Add(c1.ID, c1)
	store.Add(c2.ID, c2)

	d := &Daemon{root: tmpDir, containers: store}
	db, err := graphdb.NewSqliteConn(filepath.Join(d.root, "linkgraph.db"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Set("/"+name1, c1.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Set("/"+name2, c2.ID); err != nil {
		t.Fatal(err)
	}

	alias := "hello"
	if _, err := db.Set(path.Join(c1.Name, alias), c2.ID); err != nil {
		t.Fatal(err)
	}

	if err := d.migrateLegacySqliteLinks(db, c1); err != nil {
		t.Fatal(err)
	}

	if len(c1.HostConfig.Links) != 1 {
		t.Fatal("expected links to be populated but is empty")
	}

	expected := name2 + ":" + alias
	actual := c1.HostConfig.Links[0]
	if actual != expected {
		t.Fatalf("got wrong link value, expected: %q, got: %q", expected, actual)
	}

	// ensure this is persisted
	b, err := ioutil.ReadFile(filepath.Join(c1.Root, "hostconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	type hc struct {
		Links []string
	}
	var cfg hc
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Links) != 1 {
		t.Fatalf("expected one entry in links, got: %d", len(cfg.Links))
	}
	if cfg.Links[0] != expected { // same expected as above
		t.Fatalf("got wrong link value, expected: %q, got: %q", expected, cfg.Links[0])
	}
}
