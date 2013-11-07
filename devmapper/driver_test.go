package devmapper

import (
	"io/ioutil"
	"os"
	"testing"
)

func mkTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "docker-test-devmapper-")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInit(t *testing.T) {
	home := mkTestDirectory(t)
	defer os.RemoveAll(home)
	driver, err := Init(home)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		return
		if err := driver.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()
	id := "foo"
	if err := driver.Create(id, ""); err != nil {
		t.Fatal(err)
	}
	dir, err := driver.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if !st.IsDir() {
		t.Fatalf("Get(%V) did not return a directory", id)
	}
}
