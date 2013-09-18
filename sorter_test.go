package docker

import (
	"fmt"
	"testing"
)

func TestServerListOrderedImagesByCreationDate(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.graph.Create(archive, nil, "Testing", "", nil)
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
	image, err := runtime.graph.Create(archive, nil, "Testing", "", nil)
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

func TestSortUniquePorts(t *testing.T) {
	ports := []Port{
		Port("6379/tcp"),
		Port("22/tcp"),
	}

	sortPorts(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if fmt.Sprint(first) != "22/tcp" {
		t.Log(fmt.Sprint(first))
		t.Fail()
	}
}

func TestSortSamePortWithDifferentProto(t *testing.T) {
	ports := []Port{
		Port("8888/tcp"),
		Port("8888/udp"),
		Port("6379/tcp"),
		Port("6379/udp"),
	}

	sortPorts(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if fmt.Sprint(first) != "6379/tcp" {
		t.Fail()
	}
}
