package layer

import (
	"io"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestEmptyLayer(c *check.C) {
	if EmptyLayer.ChainID() != ChainID(DigestSHA256EmptyTar) {
		c.Fatal("wrong ID for empty layer")
	}

	if EmptyLayer.DiffID() != DigestSHA256EmptyTar {
		c.Fatal("wrong DiffID for empty layer")
	}

	if EmptyLayer.Parent() != nil {
		c.Fatal("expected no parent for empty layer")
	}

	if size, err := EmptyLayer.Size(); err != nil || size != 0 {
		c.Fatal("expected zero size for empty layer")
	}

	if diffSize, err := EmptyLayer.DiffSize(); err != nil || diffSize != 0 {
		c.Fatal("expected zero diffsize for empty layer")
	}

	tarStream, err := EmptyLayer.TarStream()
	if err != nil {
		c.Fatalf("error streaming tar for empty layer: %v", err)
	}

	digester := digest.Canonical.New()
	_, err = io.Copy(digester.Hash(), tarStream)

	if err != nil {
		c.Fatalf("error hashing empty tar layer: %v", err)
	}

	if digester.Digest() != digest.Digest(DigestSHA256EmptyTar) {
		c.Fatal("empty layer tar stream hashes to wrong value")
	}
}
