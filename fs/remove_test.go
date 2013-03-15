package fs

import (
	"github.com/dotcloud/docker/fake"
	"testing"
)

func countImages(store *Store) int {
	paths, err := store.Images()
	if err != nil {
		panic(err)
	}
	return len(paths)
}

func TestRemoveInPath(t *testing.T) {
	store, err := TempStore("test-remove-in-path")
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(store)
	archive, err := fake.FakeTar()
	if err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 0 {
		t.Fatalf("Expected 0 images, %d found", c)
	}
	images := make([]*Image, 10)
	for i := 0; i < 10; i++ {
		if image, err := store.Create(archive, nil, "foo", "Testing"); err != nil {
			t.Fatal(err)
		} else {
			images[i] = image
		}
	}
	if c := countImages(store); c != 10 {
		t.Fatalf("Expected 10 images, %d found", c)
	}
	if err := store.RemoveInPath("foo"); err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 0 {
		t.Fatalf("Expected 0 images, %d found", c)
	}
}

func TestRemove(t *testing.T) {
	store, err := TempStore("test-remove")
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(store)
	archive, err := fake.FakeTar()
	if err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 0 {
		t.Fatalf("Expected 0 images, %d found", c)
	}

	// Test 1 create / 1 delete
	img, err := store.Create(archive, nil, "foo", "Testing")
	if err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 1 {
		t.Fatalf("Expected 1 images, %d found", c)
	}
	if err := store.Remove(img); err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 0 {
		t.Fatalf("Expected 0 images, %d found", c)
	}

	// Test 2 create (same name) / 1 delete
	img1, err := store.Create(archive, nil, "foo", "Testing")
	if err != nil {
		t.Fatal(err)
	}
	img2, err := store.Create(archive, nil, "foo", "Testing")
	if err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 2 {
		t.Fatalf("Expected 2 images, %d found", c)
	}
	if err := store.Remove(img1); err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 1 {
		t.Fatalf("Expected 1 images, %d found", c)
	}

	// Test delete wrong name
	// Note: If we change orm and Delete of non existing return error, we will need to change this test
	if err := store.Remove(&Image{Id: "Not_foo", store: img2.store}); err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 1 {
		t.Fatalf("Expected 1 images, %d found", c)
	}

	// Test delete last one
	if err := store.Remove(img2); err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 0 {
		t.Fatalf("Expected 0 images, %d found", c)
	}
}

// FIXME: Do more extensive test (ex: with full name, wrong name, with Id, etc)
func TestRemoveRegexp(t *testing.T) {
	store, err := TempStore("test-remove-regexp")
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(store)
	archive, err := fake.FakeTar()
	if err != nil {
		t.Fatal(err)
	}
	if c := countImages(store); c != 0 {
		t.Fatalf("Expected 0 images, %d found", c)
	}
	_, err = store.Create(archive, nil, "foo", "Testing")
	if err != nil {
		t.Fatal(err)
	}
}
