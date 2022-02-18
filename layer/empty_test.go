package layer // import "github.com/moby/moby/layer"

import (
	"io"
	"testing"

	digest "github.com/opencontainers/go-digest"
)

func TestEmptyLayer(t *testing.T) {
	if EmptyLayer.ChainID() != ChainID(DigestSHA256EmptyTar) {
		t.Fatal("wrong ChainID for empty layer")
	}

	if EmptyLayer.DiffID() != DigestSHA256EmptyTar {
		t.Fatal("wrong DiffID for empty layer")
	}

	if EmptyLayer.Parent() != nil {
		t.Fatal("expected no parent for empty layer")
	}

	if size := EmptyLayer.Size(); size != 0 {
		t.Fatal("expected zero size for empty layer")
	}

	if diffSize := EmptyLayer.DiffSize(); diffSize != 0 {
		t.Fatal("expected zero diffsize for empty layer")
	}

	meta, err := EmptyLayer.Metadata()

	if len(meta) != 0 || err != nil {
		t.Fatal("expected zero length metadata for empty layer")
	}

	tarStream, err := EmptyLayer.TarStream()
	if err != nil {
		t.Fatalf("error streaming tar for empty layer: %v", err)
	}

	digester := digest.Canonical.Digester()
	_, err = io.Copy(digester.Hash(), tarStream)

	if err != nil {
		t.Fatalf("error hashing empty tar layer: %v", err)
	}

	if digester.Digest() != digest.Digest(DigestSHA256EmptyTar) {
		t.Fatal("empty layer tar stream hashes to wrong value")
	}
}
