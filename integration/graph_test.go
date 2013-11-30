package docker

import (
	"errors"
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/graphdriver"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestMount(t *testing.T) {
	graph, driver := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	defer driver.Cleanup()

	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := graph.Create(archive, nil, "Testing", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempDir("", "docker-test-graph-mount-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	rootfs := path.Join(tmp, "rootfs")
	if err := os.MkdirAll(rootfs, 0700); err != nil {
		t.Fatal(err)
	}
	rw := path.Join(tmp, "rw")
	if err := os.MkdirAll(rw, 0700); err != nil {
		t.Fatal(err)
	}

	if _, err := driver.Get(image.ID); err != nil {
		t.Fatal(err)
	}
}

func TestInit(t *testing.T) {
	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	// Root should exist
	if _, err := os.Stat(graph.Root); err != nil {
		t.Fatal(err)
	}
	// Map() should be empty
	if l, err := graph.Map(); err != nil {
		t.Fatal(err)
	} else if len(l) != 0 {
		t.Fatalf("len(Map()) should return %d, not %d", 0, len(l))
	}
}

// Test that Register can be interrupted cleanly without side effects
func TestInterruptedRegister(t *testing.T) {
	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	badArchive, w := io.Pipe() // Use a pipe reader as a fake archive which never yields data
	image := &docker.Image{
		ID:      docker.GenerateID(),
		Comment: "testing",
		Created: time.Now(),
	}
	w.CloseWithError(errors.New("But I'm not a tarball!")) // (Nobody's perfect, darling)
	graph.Register(nil, badArchive, image)
	if _, err := graph.Get(image.ID); err == nil {
		t.Fatal("Image should not exist after Register is interrupted")
	}
	// Registering the same image again should succeed if the first register was interrupted
	goodArchive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Register(nil, goodArchive, image); err != nil {
		t.Fatal(err)
	}
}

// FIXME: Do more extensive tests (ex: create multiple, delete, recreate;
//       create multiple, check the amount of images and paths, etc..)
func TestGraphCreate(t *testing.T) {
	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := graph.Create(archive, nil, "Testing", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := docker.ValidateID(image.ID); err != nil {
		t.Fatal(err)
	}
	if image.Comment != "Testing" {
		t.Fatalf("Wrong comment: should be '%s', not '%s'", "Testing", image.Comment)
	}
	if image.DockerVersion != docker.VERSION {
		t.Fatalf("Wrong docker_version: should be '%s', not '%s'", docker.VERSION, image.DockerVersion)
	}
	images, err := graph.Map()
	if err != nil {
		t.Fatal(err)
	} else if l := len(images); l != 1 {
		t.Fatalf("Wrong number of images. Should be %d, not %d", 1, l)
	}
	if images[image.ID] == nil {
		t.Fatalf("Could not find image with id %s", image.ID)
	}
}

func TestRegister(t *testing.T) {
	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image := &docker.Image{
		ID:      docker.GenerateID(),
		Comment: "testing",
		Created: time.Now(),
	}
	err = graph.Register(nil, archive, image)
	if err != nil {
		t.Fatal(err)
	}
	if images, err := graph.Map(); err != nil {
		t.Fatal(err)
	} else if l := len(images); l != 1 {
		t.Fatalf("Wrong number of images. Should be %d, not %d", 1, l)
	}
	if resultImg, err := graph.Get(image.ID); err != nil {
		t.Fatal(err)
	} else {
		if resultImg.ID != image.ID {
			t.Fatalf("Wrong image ID. Should be '%s', not '%s'", image.ID, resultImg.ID)
		}
		if resultImg.Comment != image.Comment {
			t.Fatalf("Wrong image comment. Should be '%s', not '%s'", image.Comment, resultImg.Comment)
		}
	}
}

// Test that an image can be deleted by its shorthand prefix
func TestDeletePrefix(t *testing.T) {
	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	img := createTestImage(graph, t)
	if err := graph.Delete(utils.TruncateID(img.ID)); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 0)
}

func createTestImage(graph *docker.Graph, t *testing.T) *docker.Image {
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img, err := graph.Create(archive, nil, "Test image", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestDelete(t *testing.T) {
	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 0)
	img, err := graph.Create(archive, nil, "Bla bla", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 1)
	if err := graph.Delete(img.ID); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 0)

	archive, err = fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	// Test 2 create (same name) / 1 delete
	img1, err := graph.Create(archive, nil, "Testing", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	archive, err = fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = graph.Create(archive, nil, "Testing", "", nil); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 2)
	if err := graph.Delete(img1.ID); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 1)

	// Test delete wrong name
	if err := graph.Delete("Not_foo"); err == nil {
		t.Fatalf("Deleting wrong ID should return an error")
	}
	assertNImages(graph, t, 1)

	archive, err = fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	// Test delete twice (pull -> rm -> pull -> rm)
	if err := graph.Register(nil, archive, img1); err != nil {
		t.Fatal(err)
	}
	if err := graph.Delete(img1.ID); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 1)
}

func TestByParent(t *testing.T) {
	archive1, _ := fakeTar()
	archive2, _ := fakeTar()
	archive3, _ := fakeTar()

	graph, _ := tempGraph(t)
	defer nukeGraph(graph)
	parentImage := &docker.Image{
		ID:      docker.GenerateID(),
		Comment: "parent",
		Created: time.Now(),
		Parent:  "",
	}
	childImage1 := &docker.Image{
		ID:      docker.GenerateID(),
		Comment: "child1",
		Created: time.Now(),
		Parent:  parentImage.ID,
	}
	childImage2 := &docker.Image{
		ID:      docker.GenerateID(),
		Comment: "child2",
		Created: time.Now(),
		Parent:  parentImage.ID,
	}
	_ = graph.Register(nil, archive1, parentImage)
	_ = graph.Register(nil, archive2, childImage1)
	_ = graph.Register(nil, archive3, childImage2)

	byParent, err := graph.ByParent()
	if err != nil {
		t.Fatal(err)
	}
	numChildren := len(byParent[parentImage.ID])
	if numChildren != 2 {
		t.Fatalf("Expected 2 children, found %d", numChildren)
	}
}

/*
 * HELPER FUNCTIONS
 */

func assertNImages(graph *docker.Graph, t *testing.T, n int) {
	if images, err := graph.Map(); err != nil {
		t.Fatal(err)
	} else if actualN := len(images); actualN != n {
		t.Fatalf("Expected %d images, found %d", n, actualN)
	}
}

func tempGraph(t *testing.T) (*docker.Graph, graphdriver.Driver) {
	tmp, err := ioutil.TempDir("", "docker-graph-")
	if err != nil {
		t.Fatal(err)
	}
	driver, err := graphdriver.New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := docker.NewGraph(tmp, driver)
	if err != nil {
		t.Fatal(err)
	}
	return graph, driver
}

func nukeGraph(graph *docker.Graph) {
	graph.Driver().Cleanup()
	os.RemoveAll(graph.Root)
}

func testArchive(t *testing.T) archive.Archive {
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	return archive
}
