package blobstore

import (
	"encoding/json"
	"os"
)

// Get the blob with the given digest from this store. Returns a nil Error
// on success.
func (ls *localStore) Get(digest string) (Blob, error) {
	info, err := ls.getBlobInfo(digest)
	if err != nil {
		return nil, err
	}

	return newBlob(newDescriptor(info), ls.blobFilename(digest)), nil
}

// getBlobInfo decodes the blob info JSON file stored beside the blob with the
// given digest.
func (ls *localStore) getBlobInfo(digest string) (info blobInfo, err *storeError) {
	blobInfoFilename := ls.blobInfoFilename(digest)
	blobInfoFile, e := os.Open(blobInfoFilename)
	if e != nil {
		if os.IsNotExist(e) {
			return info, newError(errCodeBlobNotExists, digest)
		}
		return info, newError(errCodeCannotOpenBlobInfo, e.Error())
	}
	defer blobInfoFile.Close()

	decoder := json.NewDecoder(blobInfoFile)
	if e := decoder.Decode(&info); err != nil {
		return info, newError(errCodeCannotDecodeBlobInfo, e.Error())
	}

	return info, nil
}
