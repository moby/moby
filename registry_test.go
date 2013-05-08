package docker

import (
	"os"
	"testing"
)

func TestPull(t* testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	err = runtime.graph.PullRepository(os.Stdout, "busybox", "", runtime.repositories, nil)
	if err != nil {
		t.Fatal(err)
	}
	img, err := runtime.repositories.LookupImage("busybox")
	if err != nil {
		t.Fatal(err)
	}

	// Try to run something on this image to make sure the layer's been downloaded properly.
	config, err := ParseRun([]string{img.Id, "echo", "Hello World"}, os.Stdout, runtime.capabilities)
	if err != nil {
		t.Fatal(err)
	}

	b := NewBuilder(runtime)
	container, err := b.Create(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}

	if status := container.Wait(); status != 0 {
		t.Fatalf("Expected status code 0, found %d instead", status)
	}
}

func TestPullTag(t* testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	err = runtime.graph.PullRepository(os.Stdout, "ubuntu", "12.04", runtime.repositories, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.repositories.LookupImage("ubuntu:12.04")
	if err != nil {
		t.Fatal(err)
	}

	img2, err := runtime.repositories.LookupImage("ubuntu:12.10")
	if img2 != nil {
		t.Fatalf("Expected nil image but found %v instead", img2.Id)
	}
}