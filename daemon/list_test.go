package daemon

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/pkg/graphdb"
)

func TestContainersReturnsEmptyArray(t *testing.T) {
	tmp, err := ioutil.TempDir("", "docker-daemon-list-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	graph, err := graphdb.NewSqliteConn(path.Join(tmp, "daemon_test.db"))
	if err != nil {
		t.Fatalf("Failed to create daemon test sqlite database.")
	}

	store := &contStore{s: map[string]*Container{}}

	daemon := &Daemon{
		repository:       tmp,
		root:             tmp,
		containerGraphDB: graph,
		containers:       store,
	}

	containerConfig := &ContainersConfig{
		All:     true,
		Since:   "",
		Before:  "",
		Limit:   -1,
		Size:    false,
		Filters: "",
	}

	containers, err := daemon.Containers(containerConfig)
	if err != nil {
		t.Fatal(err)
	}

	if containers == nil {
		t.Errorf("Expected empty array of containers, got nil")
	}

	if len(containers) != 0 {
		t.Errorf("Expected empty array of containers, length was %d", len(containers))
	}
}
