package docker

import (
	"github.com/dotcloud/docker/utils"
	"os"
	"path"
	"testing"
)

const (
	testImageName string = "myapp"
	testImageID   string = "foo"
)

func mkTestTagStore(root string, t *testing.T) *TagStore {
	graph, err := NewGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewTagStore(path.Join(root, "tags"), graph)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img := &Image{ID: testImageID}
	if err := graph.Register(nil, archive, img); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(testImageName, "", testImageID, false); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestLookupImage(t *testing.T) {
	tmp, err := utils.TestDirectory("")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	store := mkTestTagStore(tmp, t)

	if img, err := store.LookupImage(testImageName); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}
	if img, err := store.LookupImage(testImageName + ":" + DEFAULTTAG); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := store.LookupImage(testImageName + ":" + "fail"); err == nil {
		t.Errorf("Expected error, none found")
	} else if img != nil {
		t.Errorf("Expected 0 image, 1 found")
	}

	if img, err := store.LookupImage("fail:fail"); err == nil {
		t.Errorf("Expected error, none found")
	} else if img != nil {
		t.Errorf("Expected 0 image, 1 found")
	}

	if img, err := store.LookupImage(testImageID); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}

	if img, err := store.LookupImage(testImageName + ":" + testImageID); err != nil {
		t.Fatal(err)
	} else if img == nil {
		t.Errorf("Expected 1 image, none found")
	}
}
