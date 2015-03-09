package image

import (
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/jlhawn/blobstore"
)

type Graph interface {
	Get(id string) (*Image, error)
	ImageRoot(id string) string
	Driver() graphdriver.Driver
	BlobStore() blobstore.Store
	SetDiffDigest(id, digest string)
	DiffDigest(id string) (string, bool)
}
