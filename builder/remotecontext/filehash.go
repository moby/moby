package remotecontext

import (
	"archive/tar"
	"crypto/sha256"
	"hash"
	"os"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/tarsum"
)

// NewFileHash returns new hash that is used for the builder cache keys
func NewFileHash(path, name string, fi os.FileInfo) (hash.Hash, error) {
	hdr, err := archive.FileInfoHeader(path, name, fi)
	if err != nil {
		return nil, err
	}
	tsh := &tarsumHash{hdr: hdr, Hash: sha256.New()}
	tsh.Reset() // initialize header
	return tsh, nil
}

type tarsumHash struct {
	hash.Hash
	hdr *tar.Header
}

// Reset resets the Hash to its initial state.
func (tsh *tarsumHash) Reset() {
	// comply with hash.Hash and reset to the state hash had before any writes
	tsh.Hash.Reset()
	tarsum.WriteV1Header(tsh.hdr, tsh.Hash)
}
