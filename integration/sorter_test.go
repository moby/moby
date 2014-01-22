package docker

import (
	"github.com/dotcloud/docker"
	"testing"
	"time"
)

func TestServerListOrderedImagesByCreationDate(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	if err := generateImage("", srv); err != nil {
		t.Fatal(err)
	}

	images := getImages(eng, t, true, "")

	if images.Data[0].GetInt("Created") < images.Data[1].GetInt("Created") {
		t.Error("Expected images to be ordered by most recent creation date.")
	}
}

func TestServerListOrderedImagesByCreationDateAndTag(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkRuntimeFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	err := generateImage("bar", srv)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	err = generateImage("zed", srv)
	if err != nil {
		t.Fatal(err)
	}

	images := getImages(eng, t, true, "")

	if repoTags := images.Data[0].GetList("RepoTags"); repoTags[0] != "repo:zed" && repoTags[0] != "repo:bar" {
		t.Errorf("Expected Images to be ordered by most recent creation date.")
	}
}

func generateImage(name string, srv *docker.Server) error {
	archive, err := fakeTar()
	if err != nil {
		return err
	}
	job := srv.Eng.Job("import", "-", "repo", name)
	job.Stdin.Add(archive)
	job.SetenvBool("json", true)
	return job.Run()
}
