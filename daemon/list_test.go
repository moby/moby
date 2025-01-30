package daemon

import (
	"context"
	"fmt"
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
	c.Created = time.Now()

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

func TestContainerList(t *testing.T) {
	db, err := container.NewViewDB()
	assert.NilError(t, err)
	d := &Daemon{
		containersReplica: db,
	}

	// test list with different number of containers
	for _, num := range []int{0, 1, 2, 4, 8, 16, 32, 64, 100} {
		t.Run(fmt.Sprintf("%d containers", num), func(t *testing.T) {
			db, err := container.NewViewDB() // new DB to ignore prior containers
			assert.NilError(t, err)
			d = &Daemon{
				containersReplica: db,
			}

			// create the containers
			containers := make([]*container.Container, num)
			for i := range num {
				name := fmt.Sprintf("cont-%d", i)
				containers[i] = setupContainerWithName(t, name, d)
				// ensure container timestamps are separated enough so the
				// sort used by d.Containers() can deterministically sort them.
				containers[i].Created = containers[i].Created.Add(time.Millisecond)
			}

			// list them and verify correctness
			containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{All: true})
			assert.NilError(t, err)
			assert.Assert(t, is.Len(containerList, num))

			for i := range num {
				// container list should be ordered in descending creation order
				assert.Assert(t, is.Equal(containerList[i].Names[0], containers[num-1-i].Name))
			}
		})
	}
}

func TestContainerList_InvalidFilter(t *testing.T) {
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

func TestContainerList_NameFilter(t *testing.T) {
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

func TestContainerList_LimitFilter(t *testing.T) {
	db, err := container.NewViewDB()
	assert.NilError(t, err)
	d := &Daemon{
		containersReplica: db,
	}

	// start containers
	num := 32
	containers := make([]*container.Container, num)
	for i := range num {
		name := fmt.Sprintf("cont-%d", i)
		containers[i] = setupContainerWithName(t, name, d)
	}

	t.Run("limit 1 container", func(t *testing.T) {
		limit := 1
		containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{Limit: limit})
		assert.NilError(t, err)
		expectedListLen := min(num, limit)
		assert.Assert(t, is.Len(containerList, expectedListLen))
	})

	t.Run("limit less than num containers", func(t *testing.T) {
		limit := 20
		containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{Limit: limit})
		assert.NilError(t, err)
		expectedListLen := min(num, limit)
		assert.Assert(t, is.Len(containerList, expectedListLen))
	})

	t.Run("limit equal num containers", func(t *testing.T) {
		limit := 32
		containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{Limit: limit})
		assert.NilError(t, err)
		expectedListLen := min(num, limit)
		assert.Assert(t, is.Len(containerList, expectedListLen))
	})

	t.Run("limit greater than num containers", func(t *testing.T) {
		limit := 40
		containerList, err := d.Containers(context.Background(), &containertypes.ListOptions{Limit: limit})
		assert.NilError(t, err)
		expectedListLen := min(num, limit)
		assert.Assert(t, is.Len(containerList, expectedListLen))
	})
}
