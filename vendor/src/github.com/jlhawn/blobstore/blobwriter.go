package blobstore

import (
	"crypto"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"

	// Imported to support sha256, sha384, and sha512.
	_ "crypto/sha256"
	_ "crypto/sha512"
)

var hashLabels = map[crypto.Hash]string{
	crypto.SHA256: "sha256",
	crypto.SHA384: "sha384",
	crypto.SHA512: "sha512",
}

// HashForLabel return a crypto.Hash for the given digest label, e.g.,
// "sha256". If the hash is not supported, `crypto.Hash(0)` is returned.
// Currently, only "sha256", "sha384", and "sha512" are supported.
func HashForLabel(label string) (h crypto.Hash) {
	switch label {
	case "sha256":
		h = crypto.SHA256
	case "sha384":
		h = crypto.SHA384
	case "sha512":
		h = crypto.SHA512
	}

	if _, ok := hashLabels[h]; !(ok && h.Available()) {
		return crypto.Hash(0)
	}

	return h
}

// NewWriter begins the process of writing a new blob. If an error is returned,
// it is of type *Error.
func (ls *localStore) NewWriter(h crypto.Hash) (bw BlobWriter, err error) {
	hashLabel, ok := hashLabels[h]
	if !(ok && h.Available()) {
		return nil, newError(errCodeHashNotSupported, fmt.Sprintf("crypto.Hash(%d)", h))
	}

	tempFile, err := ioutil.TempFile(ls.root, "temp-blob-")
	if err != nil {
		return nil, newError(errCodeCannotMakeTempBlobFile, err.Error())
	}

	hasher := h.New()

	return &blobWriter{
		Writer:    io.MultiWriter(tempFile, hasher),
		hasher:    hasher,
		hashLabel: hashLabel,
		tempFile:  tempFile,
		store:     ls,
	}, nil
}

type blobWriter struct {
	io.Writer

	hasher    hash.Hash
	hashLabel string
	tempFile  *os.File
	store     *localStore
}

func (bw *blobWriter) Digest() string {
	return fmt.Sprintf("%s:%x", bw.hashLabel, bw.hasher.Sum(nil))
}

func (bw *blobWriter) Commit(mediaType, refID string) (d Descriptor, err error) {
	defer bw.Cancel()

	// Close the tempFile.
	if err = bw.tempFile.Close(); err != nil {
		return nil, newError(errCodeCannotCloseTempBlobFile, err.Error())
	}

	digest := bw.Digest()

	bw.store.Lock()
	defer bw.store.Unlock()

	// Is there already a blob with this digest in the store?
	d, blobErr := bw.store.ref(digest, refID)
	if blobErr == nil {
		return d, nil
	}

	if !blobErr.IsBlobNotExists() {
		return nil, newError(errCodeUnexpected, blobErr.Error())
	}

	// Need to get the size of the blob.
	stat, err := os.Stat(bw.tempFile.Name())
	if err != nil {
		return nil, newError(errCodeCannotStatTempBlobFile, err.Error())
	}

	info := blobInfo{
		Digest:     digest,
		MediaType:  mediaType,
		Size:       uint64(stat.Size()),
		References: []string{refID},
	}

	// Blow away this new blob directory if there's any error later.
	defer func() {
		if err != nil {
			os.RemoveAll(bw.store.blobDirname(digest))
		}
	}()

	if blobErr = bw.store.putBlobInfo(info); blobErr != nil {
		return nil, blobErr
	}

	blobFilename := bw.store.blobFilename(info.Digest)
	if err = os.Rename(bw.tempFile.Name(), blobFilename); err != nil {
		return nil, newError(errCodeCannotRenameTempBlobFile, err.Error())
	}

	return newDescriptor(info), nil
}

func (bw *blobWriter) Cancel() error {
	if err := os.Remove(bw.tempFile.Name()); err != nil {
		return newError(errCodeCannotRemoveTempBlobFile, err.Error())
	}

	return nil
}

func (ls *localStore) putBlobInfo(info blobInfo) *storeError {
	blobDirname := ls.blobDirname(info.Digest)
	if err := os.MkdirAll(blobDirname, os.FileMode(0755)); err != nil {
		return newError(errCodeCannotMakeBlobDir, err.Error())
	}

	blobInfoFilename := ls.blobInfoFilename(info.Digest)
	blobInfoFile, err := os.OpenFile(blobInfoFilename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(0600))
	if err != nil {
		return newError(errCodeCannotOpenBlobInfo, err.Error())
	}
	defer blobInfoFile.Close()

	encoder := json.NewEncoder(blobInfoFile)
	if err := encoder.Encode(info); err != nil {
		return newError(errCodeCannotEncodeBlobInfo, err.Error())
	}

	return nil
}
