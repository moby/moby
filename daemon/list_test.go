package daemon

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

var root string

func TestMain(m *testing.M) {
	var err error
	root, err = os.MkdirTemp("", "docker-container-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)

	rand.Seed(time.Now().UnixNano())

	os.Exit(m.Run())
}

// This sets up a container with a name so that name filters
// work against it. It takes in a pointer to Daemon so that
// minor operations are not repeated by the caller
func setupContainerWithName(t *testing.T, name string, daemon *Daemon) *container.Container {
	t.Helper()
	var (
		id              = uuid.New().String()
		computedImageID = image.ID(digest.FromString(id))
		cRoot           = filepath.Join(root, id)
	)
	if err := os.MkdirAll(cRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	c := container.NewBaseContainer(id, cRoot)
	// these are for passing includeContainerInList
	if name[0] != '/' {
		name = "/" + name
	}
	c.Name = name
	c.Running = true
	c.HostConfig = &containertypes.HostConfig{}

	// these are for passing the refreshImage reducer
	c.ImageID = computedImageID
	c.Config = &containertypes.Config{
		Image: computedImageID.String(),
	}

	// this is done here to avoid requiring these
	// operations n x number of containers in the
	// calling function
	daemon.containersReplica.Save(c)
	daemon.reserveName(id, name)

	return c
}

func containerListContainsName(containers []*containertypes.Summary, name string) bool {
	for _, ctr := range containers {
		for _, containerName := range ctr.Names {
			if containerName == name {
				return true
			}
		}
	}

	return false
}

func TestList(t *testing.T) {
	db, err := container.NewViewDB()
	assert.NilError(t, err)
	d := &Daemon{
		containersReplica: db,
	}

	// test list with no containers
	containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerList, 0))

	// test list of one container
	one := setupContainerWithName(t, "a1", d)
	containerList, err = d.Containers(context.Background(), &containertypes.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerList, 1))
	assert.Assert(t, containerListContainsName(containerList, one.Name))

	// test list with random number of containers
	db2, err := container.NewViewDB() // new DB to ignore prior containers
	assert.NilError(t, err)
	d = &Daemon{
		containersReplica: db2,
	}

	// start a random number of containers (between 0->256)
	num := rand.Intn(256)
	containers := make([]*container.Container, num)
	for i := range num {
		name := fmt.Sprintf("cont-%d", i)
		containers[i] = setupContainerWithName(t, name, d)
	}

	// list them and verify correctness
	containerList, err = d.Containers(context.Background(), &containertypes.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerList, num))
	for i := range num {
		assert.Assert(t, containerListContainsName(containerList, containers[i].Name))
	}
}

func TestListInvalidFilter(t *testing.T) {
	db, err := container.NewViewDB()
	assert.NilError(t, err)
	d := &Daemon{
		containersReplica: db,
	}

	_, err = d.Containers(context.Background(), &containertypes.ListOptions{
		Filters: filters.NewArgs(filters.Arg("invalid", "foo")),
	})
	assert.Assert(t, is.Error(err, "invalid filter 'invalid'"))
}

func TestNameFilter(t *testing.T) {
	db, err := container.NewViewDB()
	assert.NilError(t, err)
	d := &Daemon{
		containersReplica: db,
	}

	var (
		one   = setupContainerWithName(t, "a1", d)
		two   = setupContainerWithName(t, "a2", d)
		three = setupContainerWithName(t, "b1", d)
	)

	// moby/moby #37453 - ^ regex not working due to prefix slash
	// not being stripped
	containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "^a")),
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerList, 2))
	assert.Assert(t, containerListContainsName(containerList, one.Name))
	assert.Assert(t, containerListContainsName(containerList, two.Name))

	// Same as above but with slash prefix should produce the same result
	containerListWithPrefix, err := d.Containers(context.Background(), &containertypes.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "^/a")),
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerListWithPrefix, 2))
	assert.Assert(t, containerListContainsName(containerListWithPrefix, one.Name))
	assert.Assert(t, containerListContainsName(containerListWithPrefix, two.Name))

	// Same as above but make sure it works for exact names
	containerList, err = d.Containers(context.Background(), &containertypes.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "b1")),
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerList, 1))
	assert.Assert(t, containerListContainsName(containerList, three.Name))

	// Same as above but with slash prefix should produce the same result
	containerListWithPrefix, err = d.Containers(context.Background(), &containertypes.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", "/b1")),
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(containerListWithPrefix, 1))
	assert.Assert(t, containerListContainsName(containerListWithPrefix, three.Name))
}

func TestLimitFilter(t *testing.T) {
	db, err := container.NewViewDB()
	assert.NilError(t, err)
	d := &Daemon{
		containersReplica: db,
	}

	// start a number of containers
	num := rand.Intn(64)
	containers := make([]*container.Container, num)
	for i := range num {
		name := fmt.Sprintf("cont-%d", i)
		containers[i] = setupContainerWithName(t, name, d)
	}

	// list them with the limit option and verify correctness; note that the limit
	// option only limits the size of the list, but there are no guarantees as to
	// which containers are in that list (e.g., if limit is 10, it does not mean
	// the list will have the first 10 containers we created; it will have any 10
	// of the containers we created).
	limit := rand.Intn(64)
	containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{Limit: limit})
	assert.NilError(t, err)
	expectedListLen := min(num, limit)
	assert.Assert(t, is.Len(containerList, expectedListLen))
}
