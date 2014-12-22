package image

import (
	"github.com/docker/docker/storage"
)

type Graph interface {
	Get(id string) (*Image, error)
	ImageRoot(id string) string
	Driver() storage.Driver
}
