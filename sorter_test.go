package docker

import (
	"testing"
)

func TestServerListOrderedImagesByCreationDate(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.graph.Create(archive, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{runtime: runtime}

	images, err := srv.Images(true, "")
	if err != nil {
		t.Fatal(err)
	}

	if images[0].Created < images[1].Created {
		t.Error("Expected []APIImges to be ordered by most recent creation date.")
	}
}

func TestServerListOrderedImagesByCreationDateAndTag(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	image, err := runtime.graph.Create(archive, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{runtime: runtime}
	srv.ContainerTag(image.ID, "repo", "foo", false)
	srv.ContainerTag(image.ID, "repo", "bar", false)

	images, err := srv.Images(true, "")
	if err != nil {
		t.Fatal(err)
	}

	if images[0].Created != images[1].Created || images[0].Tag >= images[1].Tag {
		t.Error("Expected []APIImges to be ordered by most recent creation date and tag name.")
	}
}
