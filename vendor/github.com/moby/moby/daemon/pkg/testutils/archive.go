package testutils

import (
	"io"

	"github.com/moby/go-archive/compression"
	"github.com/opencontainers/go-digest"
)

// UncompressedTarDigest returns the canonical digest of the uncompressed tar stream.
func UncompressedTarDigest(compressedTar io.Reader) (digest.Digest, error) {
	rd, err := compression.DecompressStream(compressedTar)
	if err != nil {
		return "", err
	}

	defer rd.Close()

	digester := digest.Canonical.Digester()
	if _, err := io.Copy(digester.Hash(), rd); err != nil {
		return "", err
	}
	return digester.Digest(), nil
}
