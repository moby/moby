package image

import (
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageManifestKind string

const (
	ImageManifestKindImage       ImageManifestKind = "image"
	ImageManifestKindAttestation ImageManifestKind = "attestation"
	ImageManifestKindUnknown     ImageManifestKind = "unknown"
)

type ImageManifestSummary struct {
	// ID is the content-addressable ID of an image and is the same as the
	// digest of the image manifest.
	//
	// Required: true
	ID string `json:"Id"`

	// Descriptor is the OCI descriptor of the image.
	//
	// Required: true
	Descriptor ocispec.Descriptor `json:"Descriptor"`

	// Indicates whether all the child content (image config, layers) is available.
	//
	// Required: true
	Available bool `json:"Available"`

	// ContentSize is the size of all the locally available distributable content size.
	//
	// Required: true
	ContentSize int64 `json:"ContentSize"`

	// Kind is the kind of the image manifest.
	//
	// Required: true
	Kind ImageManifestKind `json:"Kind"`

	// Fields below are specific to the kind of the image manifest.

	// Present only if Kind == ImageManifestKindImage.
	ImageData ImageProperties `json:"ImageData,omitempty"`

	// Present only if Kind == ImageManifestKindAttestation.
	AttestationData AttestationProperties `json:"AttestationData,omitempty"`
}

type ImageProperties struct {
	// Platform is the OCI platform object describing the platform of
	//
	// Required: true
	Platform ocispec.Platform `json:"Platform"`

	// UnpackedSize is the size of the image when unpacked.
	//
	// Required: true
	UnpackedSize int64 `json:"UnpackedSize"`

	// Containers is the number of containers created from this image.
	//
	// Required: true
	Containers int64 `json:"Containers"`
}

type AttestationProperties struct {
	// For is the digest of the image manifest that this attestation is for.
	For digest.Digest `json:"For"`
}
