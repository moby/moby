package fingerprint

import (
	"github.com/dotcloud/docker/image"
	"github.com/dotcloud/docker/utils"
)

type Fingerprint struct {
	name            string            `json:"name"`
	metadata        *Image            `json:"metadata"`
	tarsum          string            `json:"tarsum"`
}


func getImageFingerprint(name string, img *image.Image) (fingerprint Fingerprint, error) {
	fingerprint := Fingerprint{}
	fingerprint.name = name
	fingerprint.metadata = img
	layer := img.TarLayer()
	fingerprint.tarsum = layer.tarsum
	return fingerprint, nil
}
