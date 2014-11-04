package graph

import (
	"bytes"
	"io"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/vfs" // import the vfs driver so it is used in the tests
	"github.com/docker/docker/image"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

const (
	testImageName = "myapp"
	testImageID   = "foo"
)

func fakeTar() (io.Reader, error) {
	uid := os.Getuid()
	gid := os.Getgid()

	content := []byte("Hello world!\n")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for _, name := range []string{"/etc/postgres/postgres.conf", "/etc/passwd", "/var/log/postgres/postgres.conf"} {
		hdr := new(tar.Header)

		// Leaving these fields blank requires root privileges
		hdr.Uid = uid
		hdr.Gid = gid

		hdr.Size = int64(len(content))
		hdr.Name = name
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		tw.Write([]byte(content))
	}
	tw.Close()
	return buf, nil
}

func mkTestTagStore(root string, t *testing.T) *TagStore {
	driver, err := graphdriver.New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewGraph(root, driver)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewTagStore(path.Join(root, "tags"), graph, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := fakeTar()
	if err != nil {
		t.Fatal(err)
	}
	img := &image.Image{ID: testImageID}
	if err := graph.Register(img, archive); err != nil {
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
	defer store.graph.driver.Cleanup()

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

func TestValidTagName(t *testing.T) {
	validTags := []string{"9", "foo", "foo-test", "bar.baz.boo"}
	for _, tag := range validTags {
		if err := ValidateTagName(tag); err != nil {
			t.Errorf("'%s' should've been a valid tag", tag)
		}
	}
}

func TestInvalidTagName(t *testing.T) {
	validTags := []string{"-9", ".foo", "-test", ".", "-"}
	for _, tag := range validTags {
		if err := ValidateTagName(tag); err == nil {
			t.Errorf("'%s' shouldn't have been a valid tag", tag)
		}
	}
}

func TestOfficialName(t *testing.T) {
	names := map[string]bool{
		"library/ubuntu":    true,
		"nonlibrary/ubuntu": false,
		"ubuntu":            true,
		"other/library":     false,
	}
	for name, isOfficial := range names {
		result := isOfficialName(name)
		if result != isOfficial {
			t.Errorf("Unexpected result for %s\n\tExpecting: %v\n\tActual: %v", name, isOfficial, result)
		}
	}
}
