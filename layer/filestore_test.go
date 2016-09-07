package layer

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/distribution/digest"
	"github.com/go-check/check"
)

func randomLayerID(seed int64) ChainID {
	r := rand.New(rand.NewSource(seed))

	return ChainID(digest.FromBytes([]byte(fmt.Sprintf("%d", r.Int63()))))
}

func newFileMetadataStore(c *check.C) (*fileMetadataStore, string, func()) {
	td, err := ioutil.TempDir("", "layers-")
	if err != nil {
		c.Fatal(err)
	}
	fms, err := NewFSMetadataStore(td)
	if err != nil {
		c.Fatal(err)
	}

	return fms.(*fileMetadataStore), td, func() {
		if err := os.RemoveAll(td); err != nil {
			c.Logf("Failed to cleanup %q: %s", td, err)
		}
	}
}

func assertNotDirectoryError(c *check.C, err error) {
	perr, ok := err.(*os.PathError)
	if !ok {
		c.Fatalf("Unexpected error %#v, expected path error", err)
	}

	if perr.Err != syscall.ENOTDIR {
		c.Fatalf("Unexpected error %s, expected %s", perr.Err, syscall.ENOTDIR)
	}
}

func (s *DockerSuite) TestCommitFailure(c *check.C) {
	fms, td, cleanup := newFileMetadataStore(c)
	defer cleanup()

	if err := ioutil.WriteFile(filepath.Join(td, "sha256"), []byte("was here first!"), 0644); err != nil {
		c.Fatal(err)
	}

	tx, err := fms.StartTransaction()
	if err != nil {
		c.Fatal(err)
	}

	if err := tx.SetSize(0); err != nil {
		c.Fatal(err)
	}

	err = tx.Commit(randomLayerID(5))
	if err == nil {
		c.Fatalf("Expected error committing with invalid layer parent directory")
	}
	assertNotDirectoryError(c, err)
}

func (s *DockerSuite) TestStartTransactionFailure(c *check.C) {
	fms, td, cleanup := newFileMetadataStore(c)
	defer cleanup()

	if err := ioutil.WriteFile(filepath.Join(td, "tmp"), []byte("was here first!"), 0644); err != nil {
		c.Fatal(err)
	}

	_, err := fms.StartTransaction()
	if err == nil {
		c.Fatalf("Expected error starting transaction with invalid layer parent directory")
	}
	assertNotDirectoryError(c, err)

	if err := os.Remove(filepath.Join(td, "tmp")); err != nil {
		c.Fatal(err)
	}

	tx, err := fms.StartTransaction()
	if err != nil {
		c.Fatal(err)
	}

	if expected := filepath.Join(td, "tmp"); strings.HasPrefix(expected, tx.String()) {
		c.Fatalf("Unexpected transaction string %q, expected prefix %q", tx.String(), expected)
	}

	if err := tx.Cancel(); err != nil {
		c.Fatal(err)
	}
}
