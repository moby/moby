package blobstore

import (
	"io/ioutil"
	"strings"
)

func (ls *localStore) List() ([]string, error) {
	dirInfos, err := ioutil.ReadDir(ls.blobDirname(""))
	if err != nil {
		return nil, newError(errCodeCannotListBlobsDir, err.Error())
	}

	blobDigests := make([]string, 0, len(dirInfos))

	for _, dirInfo := range dirInfos {
		if !(dirInfo.IsDir() && strings.HasPrefix(dirInfo.Name(), "sha256:")) {
			// Blobs are stored in directories prefixed by "sha256:".
			continue
		}

		blobDigests = append(blobDigests, dirInfo.Name())
	}

	return blobDigests, nil
}
