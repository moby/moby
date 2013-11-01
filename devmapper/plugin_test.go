package devmapper

import (
	"io/ioutil"
	_ "os"
	"testing"
)

type TestImage struct {
	id	string
	path	string
}

func (img *TestImage) ID() string {
	return img.id
}

func (img *TestImage) Path() string {
	return img.path
}

func (img *TestImage) Parent() (Image, error) {
	return nil, nil
}



func mkTestImage(t *testing.T) Image {
	return &TestImage{
		path:	mkTestDirectory(t),
		id:	"4242",
	}
}

func mkTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "docker-test-devmapper-")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInit(t *testing.T) {
	home := mkTestDirectory(t)
	// defer os.RemoveAll(home)
	plugin, err := Init(home)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		return
		if err := plugin.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()
	img := mkTestImage(t)
	// defer os.RemoveAll(img.(*TestImage).path)
	if err := plugin.Create(img, nil); err != nil {
		t.Fatal(err)
	}
}
