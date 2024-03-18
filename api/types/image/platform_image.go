package image

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type PlatformImage struct {
	// ID is the content-addressable ID of an image and is the same as the
	// digest of the platform-specific image manifest.
	//
	// Required: true
	ID string `json:"Id"`

	// Descriptor is the OCI descriptor of the image.
	//
	// Required: true
	Descriptor ocispec.Descriptor `json:"Descriptor"`

	// Available indicates whether the image is locally available.
	//
	// Required: true
	Available bool `json:"Available"`

	// Platform is the platform of the image
	//
	// Required: true
	Platform ocispec.Platform `json:"Platform"`

	// ContentSize is the size of all the locally available distributable content size.
	//
	// Required: true
	ContentSize int64 `json:"ContentSize"`

	// UnpackedSize is the size of the image when unpacked.
	//
	// Required: true
	UnpackedSize int64 `json:"UnpackedSize"`

	// Containers is the number of containers created from this image.
	//
	// Required: true
	Containers int64 `json:"Containers"`
}
