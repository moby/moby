package registry

import (
	"crypto/rand"
	"encoding/hex"
    "github.com/dotcloud/docker"
	"github.com/dotcloud/docker/auth"
    "io/ioutil"
	"os"
	"os/exec"
    "path"
    "sync"
	"testing"
)

const unitTestStoreBase string = "/var/lib/docker/unit-tests"

func CopyDirectory(source, dest string) error {
    if _, err := exec.Command("cp", "-ra", source, dest).Output(); err != nil {
        return err
    }
    return nil
}

func nuke(srv *docker.Server) error {
    var wg sync.WaitGroup
    for _, container := range srv.Runtime().List() {
        wg.Add(1)
        go func(c *docker.Container) {
            c.Kill()
            wg.Done()
        }(container)
    }
    wg.Wait()
    return os.RemoveAll(srv.Runtime().Root())
}

func newTestServer() (*docker.Server, error) {
    root, err := ioutil.TempDir("", "docker-test")
    if err != nil {
        return nil, err
    }
    if err := os.Remove(root); err != nil {
        return nil, err
    }
    if err := CopyDirectory(unitTestStoreBase, root); err != nil {
        return nil, err
    }

    srv, err := docker.NewServerFromDirectory(root, false)
    if err != nil {
        return nil, err
    }
    return srv, nil
}

func TestPull(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "")
	srv, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(srv)

	err = srv.ImagePull("busybox", "", "", ioutil.Discard)
	if err != nil {
		t.Fatal(err)
	}
	img, err := srv.ImageInspect("busybox")
	if err != nil {
		t.Fatal(err)
	}

	// Try to run something on this image to make sure the layer's been downloaded properly.
	config, _, err := docker.ParseRun([]string{img.Id, "/bin/echo", "HelloWorld"}, &docker.Capabilities{})
	if err != nil {
		t.Fatal(err)
	}

	b := docker.NewBuilder(srv.Runtime())
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
	srv, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(srv)

	err = srv.ImagePull("ubuntu", "12.04", "", ioutil.Discard)
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.ImageInspect("ubuntu:12.04")
	if err != nil {
		t.Fatal(err)
	}

	img2, err := srv.ImageInspect("ubuntu:12.10")
	if img2 != nil {
		t.Fatalf("Expected nil image but found %v instead", img2.Id)
	}
}

func login(srv *docker.Server) error {
	authConfig := auth.NewAuthConfig("unittester", "surlautrerivejetattendrai", "noise+unittester@dotcloud.com", srv.Runtime().Root())
	srv.Registry().ResetClient(authConfig)
	_, err := auth.Login(authConfig)
	return err
}

func TestPush(t *testing.T) {
	os.Setenv("DOCKER_INDEX_URL", "https://indexstaging-docker.dotcloud.com")
	defer os.Setenv("DOCKER_INDEX_URL", "")
	srv, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(srv)

	err = login(srv)
	if err != nil {
		t.Fatal(err)
	}

	err = srv.ImagePull("joffrey/busybox", "", "", ioutil.Discard)
	if err != nil {
		t.Fatal(err)
	}
	tokenBuffer := make([]byte, 16)
	_, err = rand.Read(tokenBuffer)
	if err != nil {
		t.Fatal(err)
	}
	token := hex.EncodeToString(tokenBuffer)[:29]
	config, _, err := docker.ParseRun([]string{"joffrey/busybox", "touch", "/" + token}, &docker.Capabilities{})
	if err != nil {
		t.Fatal(err)
	}

	b := docker.NewBuilder(srv.Runtime())
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

	err = srv.ImagePush("unittester/"+token, "", ioutil.Discard)
	if err != nil {
		t.Fatal(err)
	}

	// Remove image so we can pull it again
	if err := srv.ImageDelete(img.Id); err != nil {
		t.Fatal(err)
	}

	err = srv.ImagePull("unittester/"+token, "", "", ioutil.Discard)
	if err != nil {
		t.Fatal(err)
	}

	layerPath, err := img.Layer()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path.Join(layerPath, token)); err != nil {
		t.Fatalf("Error while trying to retrieve token file: %v", err)
	}
}
