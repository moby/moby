package image

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestFSGetSet(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	testGetSet(c, fs)
}

func (s *DockerSuite) TestFSGetInvalidData(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	id, err := fs.Set([]byte("foobar"))
	if err != nil {
		c.Fatal(err)
	}

	dgst := digest.Digest(id)

	if err := ioutil.WriteFile(filepath.Join(tmpdir, contentDirName, string(dgst.Algorithm()), dgst.Hex()), []byte("foobar2"), 0600); err != nil {
		c.Fatal(err)
	}

	_, err = fs.Get(id)
	if err == nil {
		c.Fatal("Expected get to fail after data modification.")
	}
}

func (s *DockerSuite) TestFSInvalidSet(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	id := digest.FromBytes([]byte("foobar"))
	err = os.Mkdir(filepath.Join(tmpdir, contentDirName, string(id.Algorithm()), id.Hex()), 0700)
	if err != nil {
		c.Fatal(err)
	}

	_, err = fs.Set([]byte("foobar"))
	if err == nil {
		c.Fatal("Expecting error from invalid filesystem data.")
	}
}

func (s *DockerSuite) TestFSInvalidRoot(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	tcases := []struct {
		root, invalidFile string
	}{
		{"root", "root"},
		{"root", "root/content"},
		{"root", "root/metadata"},
	}

	for _, tc := range tcases {
		root := filepath.Join(tmpdir, tc.root)
		filePath := filepath.Join(tmpdir, tc.invalidFile)
		err := os.MkdirAll(filepath.Dir(filePath), 0700)
		if err != nil {
			c.Fatal(err)
		}
		f, err := os.Create(filePath)
		if err != nil {
			c.Fatal(err)
		}
		f.Close()

		_, err = NewFSStoreBackend(root)
		if err == nil {
			c.Fatalf("Expected error from root %q and invlid file %q", tc.root, tc.invalidFile)
		}

		os.RemoveAll(root)
	}

}

func testMetadataGetSet(c *check.C, store StoreBackend) {
	id, err := store.Set([]byte("foo"))
	if err != nil {
		c.Fatal(err)
	}
	id2, err := store.Set([]byte("bar"))
	if err != nil {
		c.Fatal(err)
	}

	tcases := []struct {
		id    ID
		key   string
		value []byte
	}{
		{id, "tkey", []byte("tval1")},
		{id, "tkey2", []byte("tval2")},
		{id2, "tkey", []byte("tval3")},
	}

	for _, tc := range tcases {
		err = store.SetMetadata(tc.id, tc.key, tc.value)
		if err != nil {
			c.Fatal(err)
		}

		actual, err := store.GetMetadata(tc.id, tc.key)
		if err != nil {
			c.Fatal(err)
		}
		if bytes.Compare(actual, tc.value) != 0 {
			c.Fatalf("Metadata expected %q, got %q", tc.value, actual)
		}
	}

	_, err = store.GetMetadata(id2, "tkey2")
	if err == nil {
		c.Fatal("Expected error for getting metadata for unknown key")
	}

	id3 := digest.FromBytes([]byte("baz"))
	err = store.SetMetadata(ID(id3), "tkey", []byte("tval"))
	if err == nil {
		c.Fatal("Expected error for setting metadata for unknown ID.")
	}

	_, err = store.GetMetadata(ID(id3), "tkey")
	if err == nil {
		c.Fatal("Expected error for getting metadata for unknown ID.")
	}
}

func (s *DockerSuite) TestFSMetadataGetSet(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	testMetadataGetSet(c, fs)
}

func (s *DockerSuite) TestFSDelete(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	testDelete(c, fs)
}

func (s *DockerSuite) TestFSWalker(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	testWalker(c, fs)
}

func (s *DockerSuite) TestFSInvalidWalker(c *check.C) {
	tmpdir, err := ioutil.TempDir("", "images-fs-store")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	fs, err := NewFSStoreBackend(tmpdir)
	if err != nil {
		c.Fatal(err)
	}

	fooID, err := fs.Set([]byte("foo"))
	if err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(tmpdir, contentDirName, "sha256/foobar"), []byte("foobar"), 0600); err != nil {
		c.Fatal(err)
	}

	n := 0
	err = fs.Walk(func(id ID) error {
		if id != fooID {
			c.Fatalf("Invalid walker ID %q, expected %q", id, fooID)
		}
		n++
		return nil
	})
	if err != nil {
		c.Fatalf("Invalid data should not have caused walker error, got %v", err)
	}
	if n != 1 {
		c.Fatalf("Expected 1 walk initialization, got %d", n)
	}
}

func testGetSet(c *check.C, store StoreBackend) {
	type tcase struct {
		input    []byte
		expected ID
	}
	tcases := []tcase{
		{[]byte("foobar"), ID("sha256:c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2")},
	}

	randomInput := make([]byte, 8*1024)
	_, err := rand.Read(randomInput)
	if err != nil {
		c.Fatal(err)
	}
	// skipping use of digest pkg because its used by the implementation
	h := sha256.New()
	_, err = h.Write(randomInput)
	if err != nil {
		c.Fatal(err)
	}
	tcases = append(tcases, tcase{
		input:    randomInput,
		expected: ID("sha256:" + hex.EncodeToString(h.Sum(nil))),
	})

	for _, tc := range tcases {
		id, err := store.Set([]byte(tc.input))
		if err != nil {
			c.Fatal(err)
		}
		if id != tc.expected {
			c.Fatalf("Expected ID %q, got %q", tc.expected, id)
		}
	}

	for _, emptyData := range [][]byte{nil, {}} {
		_, err := store.Set(emptyData)
		if err == nil {
			c.Fatal("Expected error for nil input.")
		}
	}

	for _, tc := range tcases {
		data, err := store.Get(tc.expected)
		if err != nil {
			c.Fatal(err)
		}
		if bytes.Compare(data, tc.input) != 0 {
			c.Fatalf("Expected data %q, got %q", tc.input, data)
		}
	}

	for _, key := range []ID{"foobar:abc", "sha256:abc", "sha256:c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2a"} {
		_, err := store.Get(key)
		if err == nil {
			c.Fatalf("Expected error for ID %q.", key)
		}
	}

}

func testDelete(c *check.C, store StoreBackend) {
	id, err := store.Set([]byte("foo"))
	if err != nil {
		c.Fatal(err)
	}
	id2, err := store.Set([]byte("bar"))
	if err != nil {
		c.Fatal(err)
	}

	err = store.Delete(id)
	if err != nil {
		c.Fatal(err)
	}

	_, err = store.Get(id)
	if err == nil {
		c.Fatalf("Expected getting deleted item %q to fail", id)
	}
	_, err = store.Get(id2)
	if err != nil {
		c.Fatal(err)
	}

	err = store.Delete(id2)
	if err != nil {
		c.Fatal(err)
	}
	_, err = store.Get(id2)
	if err == nil {
		c.Fatalf("Expected getting deleted item %q to fail", id2)
	}
}

func testWalker(c *check.C, store StoreBackend) {
	id, err := store.Set([]byte("foo"))
	if err != nil {
		c.Fatal(err)
	}
	id2, err := store.Set([]byte("bar"))
	if err != nil {
		c.Fatal(err)
	}

	tcases := make(map[ID]struct{})
	tcases[id] = struct{}{}
	tcases[id2] = struct{}{}
	n := 0
	err = store.Walk(func(id ID) error {
		delete(tcases, id)
		n++
		return nil
	})
	if err != nil {
		c.Fatal(err)
	}

	if n != 2 {
		c.Fatalf("Expected 2 walk initializations, got %d", n)
	}
	if len(tcases) != 0 {
		c.Fatalf("Expected empty unwalked set, got %+v", tcases)
	}

	// stop on error
	tcases = make(map[ID]struct{})
	tcases[id] = struct{}{}
	err = store.Walk(func(id ID) error {
		return errors.New("")
	})
	if err == nil {
		c.Fatalf("Exected error from walker.")
	}
}
