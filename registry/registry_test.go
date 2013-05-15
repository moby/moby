package registry

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/dotcloud/docker/auth"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestPull(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "")
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	err = runtime.graph.PullRepository(ioutil.Discard, "busybox", "", runtime.repositories, nil)
	if err != nil {
		t.Fatal(err)
	}
	img, err := runtime.repositories.LookupImage("busybox")
	if err != nil {
		t.Fatal(err)
	}

	// Try to run something on this image to make sure the layer's been downloaded properly.
	config, _, err := ParseRun([]string{img.Id, "echo", "Hello World"}, runtime.capabilities)
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

func TestPullTag(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "")
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	err = runtime.graph.PullRepository(ioutil.Discard, "ubuntu", "12.04", runtime.repositories, nil)
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

func login(runtime *Runtime) error {
	authConfig := auth.NewAuthConfig("unittester", "surlautrerivejetattendrai", "noise+unittester@dotcloud.com", runtime.root)
	runtime.authConfig = authConfig
	_, err := auth.Login(authConfig)
	return err
}

func TestPush(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "https://indexstaging-docker.dotcloud.com")
	defer os.Setenv("DOCKER_INDEX_URL", "")
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	err = login(runtime)
	if err != nil {
		t.Fatal(err)
	}

	err = runtime.graph.PullRepository(ioutil.Discard, "joffrey/busybox", "", runtime.repositories, nil)
	if err != nil {
		t.Fatal(err)
	}
	tokenBuffer := make([]byte, 16)
	_, err = rand.Read(tokenBuffer)
	if err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(tokenBuffer)[:29]
	config, _, err := ParseRun([]string{"joffrey/busybox", "touch", "/" + token}, runtime.capabilities)
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

	img, err := b.Commit(container, "unittester/"+token, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	repo := runtime.repositories.Repositories["unittester/"+token]
	err = runtime.graph.PushRepository(ioutil.Discard, "unittester/"+token, repo, runtime.authConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Remove image so we can pull it again
	if err := runtime.graph.Delete(img.Id); err != nil {
		t.Fatal(err)
	}

	err = runtime.graph.PullRepository(ioutil.Discard, "unittester/"+token, "", runtime.repositories, runtime.authConfig)
	if err != nil {
		t.Fatal(err)
	}

	layerPath, err := img.layer()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(layerPath, token)); err != nil {
		t.Fatalf("Error while trying to retrieve token file: %v", err)
	}
}
