// Package tarsum provides algorithms to perform checksum calculation on
// filesystem layers.
//
// The transportation of filesystems, regarding Docker, is done with tar(1)
// archives. There are a variety of tar serialization formats [2], and a key
// concern here is ensuring a repeatable checksum given a set of inputs from a
// generic tar archive. Types of transportation include distribution to and from a
// registry endpoint, saving and loading through commands or Docker daemon APIs,
// transferring the build context from client to Docker daemon, and committing the
// filesystem of a container to become an image.
//
// As tar archives are used for transit, but not preserved in many situations, the
// focus of the algorithm is to ensure the integrity of the preserved filesystem,
// while maintaining a deterministic accountability. This includes neither
// constraining the ordering or manipulation of the files during the creation or
// unpacking of the archive, nor include additional metadata state about the file
// system attributes.
package tarsum // import "github.com/moby/moby/pkg/tarsum"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"path"
	"strings"
)

const (
	buf8K  = 8 * 1024
	buf16K = 16 * 1024
	buf32K = 32 * 1024
)

// NewTarSum creates a new interface for calculating a fixed time checksum of a
// tar archive.
//
// This is used for calculating checksums of layers of an image, in some cases
// including the byte payload of the image's json metadata as well, and for
// calculating the checksums for buildcache.
func NewTarSum(r io.Reader, dc bool, v Version) (TarSum, error) {
	return NewTarSumHash(r, dc, v, DefaultTHash)
}

// NewTarSumHash creates a new TarSum, providing a THash to use rather than
// the DefaultTHash.
func NewTarSumHash(r io.Reader, dc bool, v Version, tHash THash) (TarSum, error) {
	headerSelector, err := getTarHeaderSelector(v)
	if err != nil {
		return nil, err
	}
	ts := &tarSum{Reader: r, DisableCompression: dc, tarSumVersion: v, headerSelector: headerSelector, tHash: tHash}
	err = ts.initTarSum()
	return ts, err
}

// NewTarSumForLabel creates a new TarSum using the provided TarSum version+hash label.
func NewTarSumForLabel(r io.Reader, disableCompression bool, label string) (TarSum, error) {
	parts := strings.SplitN(label, "+", 2)
	if len(parts) != 2 {
		return nil, errors.New("tarsum label string should be of the form: {tarsum_version}+{hash_name}")
	}

	versionName, hashName := parts[0], parts[1]

	version, ok := tarSumVersionsByName[versionName]
	if !ok {
		return nil, fmt.Errorf("unknown TarSum version name: %q", versionName)
	}

	hashConfig, ok := standardHashConfigs[hashName]
	if !ok {
		return nil, fmt.Errorf("unknown TarSum hash name: %q", hashName)
	}

	tHash := NewTHash(hashConfig.name, hashConfig.hash.New)

	return NewTarSumHash(r, disableCompression, version, tHash)
}

// TarSum is the generic interface for calculating fixed time
// checksums of a tar archive.
type TarSum interface {
	io.Reader
	GetSums() FileInfoSums
	Sum([]byte) string
	Version() Version
	Hash() THash
}

// tarSum struct is the structure for a Version0 checksum calculation.
type tarSum struct {
	io.Reader
	tarR               *tar.Reader
	tarW               *tar.Writer
	writer             writeCloseFlusher
	bufTar             *bytes.Buffer
	bufWriter          *bytes.Buffer
	bufData            []byte
	h                  hash.Hash
	tHash              THash
	sums               FileInfoSums
	fileCounter        int64
	currentFile        string
	finished           bool
	first              bool
	DisableCompression bool              // false by default. When false, the output gzip compressed.
	tarSumVersion      Version           // this field is not exported so it can not be mutated during use
	headerSelector     tarHeaderSelector // handles selecting and ordering headers for files in the archive
}

func (ts tarSum) Hash() THash {
	return ts.tHash
}

func (ts tarSum) Version() Version {
	return ts.tarSumVersion
}

// THash provides a hash.Hash type generator and its name.
type THash interface {
	Hash() hash.Hash
	Name() string
}

// NewTHash is a convenience method for creating a THash.
func NewTHash(name string, h func() hash.Hash) THash {
	return simpleTHash{n: name, h: h}
}

type tHashConfig struct {
	name string
	hash crypto.Hash
}

var (
	// NOTE: DO NOT include MD5 or SHA1, which are considered insecure.
	standardHashConfigs = map[string]tHashConfig{
		"sha256": {name: "sha256", hash: crypto.SHA256},
		"sha512": {name: "sha512", hash: crypto.SHA512},
	}
)

// DefaultTHash is default TarSum hashing algorithm - "sha256".
var DefaultTHash = NewTHash("sha256", sha256.New)

type simpleTHash struct {
	n string
	h func() hash.Hash
}

func (sth simpleTHash) Name() string    { return sth.n }
func (sth simpleTHash) Hash() hash.Hash { return sth.h() }

func (ts *tarSum) encodeHeader(h *tar.Header) error {
	for _, elem := range ts.headerSelector.selectHeaders(h) {
		// Ignore these headers to be compatible with versions
		// before go 1.10
		if elem[0] == "gname" || elem[0] == "uname" {
			elem[1] = ""
		}
		if _, err := ts.h.Write([]byte(elem[0] + elem[1])); err != nil {
			return err
		}
	}
	return nil
}

func (ts *tarSum) initTarSum() error {
	ts.bufTar = bytes.NewBuffer([]byte{})
	ts.bufWriter = bytes.NewBuffer([]byte{})
	ts.tarR = tar.NewReader(ts.Reader)
	ts.tarW = tar.NewWriter(ts.bufTar)
	if !ts.DisableCompression {
		ts.writer = gzip.NewWriter(ts.bufWriter)
	} else {
		ts.writer = &nopCloseFlusher{Writer: ts.bufWriter}
	}
	if ts.tHash == nil {
		ts.tHash = DefaultTHash
	}
	ts.h = ts.tHash.Hash()
	ts.h.Reset()
	ts.first = true
	ts.sums = FileInfoSums{}
	return nil
}

func (ts *tarSum) Read(buf []byte) (int, error) {
	if ts.finished {
		return ts.bufWriter.Read(buf)
	}
	if len(ts.bufData) < len(buf) {
		switch {
		case len(buf) <= buf8K:
			ts.bufData = make([]byte, buf8K)
		case len(buf) <= buf16K:
			ts.bufData = make([]byte, buf16K)
		case len(buf) <= buf32K:
			ts.bufData = make([]byte, buf32K)
		default:
			ts.bufData = make([]byte, len(buf))
		}
	}
	buf2 := ts.bufData[:len(buf)]

	n, err := ts.tarR.Read(buf2)
	if err != nil {
		if err == io.EOF {
			if _, err := ts.h.Write(buf2[:n]); err != nil {
				return 0, err
			}
			if !ts.first {
				ts.sums = append(ts.sums, fileInfoSum{name: ts.currentFile, sum: hex.EncodeToString(ts.h.Sum(nil)), pos: ts.fileCounter})
				ts.fileCounter++
				ts.h.Reset()
			} else {
				ts.first = false
			}

			if _, err := ts.tarW.Write(buf2[:n]); err != nil {
				return 0, err
			}

			currentHeader, err := ts.tarR.Next()
			if err != nil {
				if err == io.EOF {
					if err := ts.tarW.Close(); err != nil {
						return 0, err
					}
					if _, err := io.Copy(ts.writer, ts.bufTar); err != nil {
						return 0, err
					}
					if err := ts.writer.Close(); err != nil {
						return 0, err
					}
					ts.finished = true
					return ts.bufWriter.Read(buf)
				}
				return 0, err
			}

			ts.currentFile = path.Join(".", path.Join("/", currentHeader.Name))
			if err := ts.encodeHeader(currentHeader); err != nil {
				return 0, err
			}
			if err := ts.tarW.WriteHeader(currentHeader); err != nil {
				return 0, err
			}

			if _, err := io.Copy(ts.writer, ts.bufTar); err != nil {
				return 0, err
			}
			ts.writer.Flush()

			return ts.bufWriter.Read(buf)
		}
		return 0, err
	}

	// Filling the hash buffer
	if _, err = ts.h.Write(buf2[:n]); err != nil {
		return 0, err
	}

	// Filling the tar writer
	if _, err = ts.tarW.Write(buf2[:n]); err != nil {
		return 0, err
	}

	// Filling the output writer
	if _, err = io.Copy(ts.writer, ts.bufTar); err != nil {
		return 0, err
	}
	ts.writer.Flush()

	return ts.bufWriter.Read(buf)
}

func (ts *tarSum) Sum(extra []byte) string {
	ts.sums.SortBySums()
	h := ts.tHash.Hash()
	if extra != nil {
		h.Write(extra)
	}
	for _, fis := range ts.sums {
		h.Write([]byte(fis.Sum()))
	}
	checksum := ts.Version().String() + "+" + ts.tHash.Name() + ":" + hex.EncodeToString(h.Sum(nil))
	return checksum
}

func (ts *tarSum) GetSums() FileInfoSums {
	return ts.sums
}
