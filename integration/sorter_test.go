package docker

import (
	"github.com/dotcloud/docker"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
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

	if images.Data[0].GetList("RepoTags")[0] != "repo:zed" && images.Data[0].GetList("RepoTags")[0] != "repo:bar" {
		t.Errorf("Expected []APIImges to be ordered by most recent creation date. %s", images)
	}
}

func generateImage(name string, srv *docker.Server) error {
	archive, err := fakeTar()
	if err != nil {
		return err
	}
	return srv.ImageImport("-", "repo", name, archive, ioutil.Discard, utils.NewStreamFormatter(true))
}
