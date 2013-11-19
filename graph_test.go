package docker

import (
	"archive/tar"
	"bytes"
	"errors"
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/graphdriver"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
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
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	badArchive, w := io.Pipe() // Use a pipe reader as a fake archive which never yields data
	image := &Image{
		ID:      GenerateID(),
		Comment: "testing",
		Created: time.Now(),
	}
	go graph.Register(nil, badArchive, image)
	time.Sleep(200 * time.Millisecond)
	w.CloseWithError(errors.New("But I'm not a tarball!")) // (Nobody's perfect, darling)
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
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := graph.Create(archive, nil, "Testing", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateID(image.ID); err != nil {
		t.Fatal(err)
	}
	if image.Comment != "Testing" {
		t.Fatalf("Wrong comment: should be '%s', not '%s'", "Testing", image.Comment)
	}
	if image.DockerVersion != VERSION {
		t.Fatalf("Wrong docker_version: should be '%s', not '%s'", VERSION, image.DockerVersion)
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
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image := &Image{
		ID:      GenerateID(),
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
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	img := createTestImage(graph, t)
	if err := graph.Delete(utils.TruncateID(img.ID)); err != nil {
		t.Fatal(err)
	}
	assertNImages(graph, t, 0)
}

func createTestImage(graph *Graph, t *testing.T) *Image {
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
	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
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

	graph := tempGraph(t)
	defer os.RemoveAll(graph.Root)
	parentImage := &Image{
		ID:      GenerateID(),
		Comment: "parent",
		Created: time.Now(),
		Parent:  "",
	}
	childImage1 := &Image{
		ID:      GenerateID(),
		Comment: "child1",
		Created: time.Now(),
		Parent:  parentImage.ID,
	}
	childImage2 := &Image{
		ID:      GenerateID(),
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

func assertNImages(graph *Graph, t *testing.T, n int) {
	if images, err := graph.Map(); err != nil {
		t.Fatal(err)
	} else if actualN := len(images); actualN != n {
		t.Fatalf("Expected %d images, found %d", n, actualN)
	}
}

/*
 * HELPER FUNCTIONS
 */

func tempGraph(t *testing.T) *Graph {
	tmp, err := ioutil.TempDir("", "docker-graph-")
	if err != nil {
		t.Fatal(err)
	}
	backend, err := graphdriver.New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewGraph(tmp, backend)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func testArchive(t *testing.T) archive.Archive {
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

func fakeTar() (io.Reader, error) {
	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)
		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}
